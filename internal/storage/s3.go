package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"machine-logbook/internal/domain"
)

type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	PathStyle bool
}

type S3 struct {
	client *s3.Client
	bucket string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("S3 bucket is required")
	}
	if cfg.Endpoint != "" {
		endpoint, err := url.Parse(cfg.Endpoint)
		if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return nil, errors.New("S3 endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
		}
	}
	var options []func(*config.LoadOptions) error
	if cfg.Region != "" {
		options = append(options, config.WithRegion(cfg.Region))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	if awsConfig.Region == "" {
		return nil, errors.New("S3 region is required")
	}
	client := s3.NewFromConfig(awsConfig, func(opts *s3.Options) {
		opts.UsePathStyle = cfg.PathStyle
		opts.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		opts.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		if cfg.Endpoint != "" {
			opts.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// A seekable body lets the SDK retry uploads without retaining photos in memory.
	file, err := os.CreateTemp("", "machine-logbook-s3-*")
	if err != nil {
		return fmt.Errorf("create S3 upload buffer: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	if err := copyExact(ctx, file, body, size); err != nil {
		return fmt.Errorf("buffer S3 object: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind S3 object: %w", err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: file,
		ContentLength: aws.Int64(size), ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload S3 object: %w", err)
	}
	return nil
}

func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isS3NotFound(err) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read S3 object: %w", err)
	}
	return output.Body, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil && !isS3NotFound(err) {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}

func (s *S3) Check(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("check S3 bucket: %w", err)
	}
	return nil
}

func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
		return true
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound
}
