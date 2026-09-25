package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment          string
	HTTPAddr             string
	DatabaseURL          string
	DatabaseMaxConns     int32
	StorageDriver        string
	LocalStoragePath     string
	S3Bucket             string
	S3Region             string
	S3Endpoint           string
	S3PathStyle          bool
	AllowedOrigins       []string
	SessionTTL           time.Duration
	RequestTimeout       time.Duration
	ShutdownTimeout      time.Duration
	MaxRequestBytes      int64
	MaxPhotoBytes        int64
	MaxImagePixels       int64
	MaxPhotosPerUpload   int
	MaxPhotosPerLogbook  int
	MaxConcurrentUploads int
}

func Load() (Config, error) {
	c := Config{
		Environment:      value("APP_ENV", "development"),
		HTTPAddr:         value("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		StorageDriver:    value("STORAGE_DRIVER", "local"),
		LocalStoragePath: value("LOCAL_STORAGE_PATH", "./data/uploads"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3Region:         value("AWS_REGION", "us-east-1"),
		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
	}
	var err error
	readInt := func(key string, fallback, min, max int64) int64 {
		n, e := strconv.ParseInt(value(key, strconv.FormatInt(fallback, 10)), 10, 64)
		if e != nil || n < min || n > max {
			err = errors.Join(err, fmt.Errorf("%s must be between %d and %d", key, min, max))
		}
		return n
	}
	readDuration := func(key string, fallback string, min, max time.Duration) time.Duration {
		d, e := time.ParseDuration(value(key, fallback))
		if e != nil || d < min || d > max {
			err = errors.Join(err, fmt.Errorf("%s must be between %s and %s", key, min, max))
		}
		return d
	}
	c.DatabaseMaxConns = int32(readInt("DATABASE_MAX_CONNS", 10, 2, 200))
	c.MaxRequestBytes = readInt("MAX_REQUEST_BYTES", 100<<20, 1<<20, 1<<30)
	c.MaxPhotoBytes = readInt("MAX_PHOTO_BYTES", 10<<20, 1024, 50<<20)
	c.MaxImagePixels = readInt("MAX_IMAGE_PIXELS", 40_000_000, 1, 100_000_000)
	c.MaxPhotosPerUpload = int(readInt("MAX_PHOTOS_PER_UPLOAD", 20, 1, 100))
	c.MaxPhotosPerLogbook = int(readInt("MAX_PHOTOS_PER_LOGBOOK", 100, 1, 1000))
	c.MaxConcurrentUploads = int(readInt("MAX_CONCURRENT_UPLOADS", 4, 1, 32))
	c.SessionTTL = readDuration("SESSION_TTL", "12h", time.Minute, 30*24*time.Hour)
	c.RequestTimeout = readDuration("REQUEST_TIMEOUT", "2m", 5*time.Second, 15*time.Minute)
	c.ShutdownTimeout = readDuration("SHUTDOWN_TIMEOUT", "30s", time.Second, 5*time.Minute)
	c.S3PathStyle, _ = strconv.ParseBool(value("S3_PATH_STYLE", "false"))
	if _, e := strconv.ParseBool(value("S3_PATH_STYLE", "false")); e != nil {
		err = errors.Join(err, errors.New("S3_PATH_STYLE must be true or false"))
	}
	if c.Environment != "development" && c.Environment != "production" && c.Environment != "test" {
		err = errors.Join(err, errors.New("APP_ENV must be development, production, or test"))
	}
	if c.DatabaseURL == "" {
		err = errors.Join(err, errors.New("DATABASE_URL is required"))
	}
	if c.StorageDriver != "local" && c.StorageDriver != "s3" {
		err = errors.Join(err, errors.New("STORAGE_DRIVER must be local or s3"))
	}
	if c.StorageDriver == "s3" && c.S3Bucket == "" {
		err = errors.Join(err, errors.New("S3_BUCKET is required for s3 storage"))
	}
	if c.MaxPhotoBytes > c.MaxRequestBytes {
		err = errors.Join(err, errors.New("MAX_PHOTO_BYTES cannot exceed MAX_REQUEST_BYTES"))
	}
	if c.MaxPhotosPerUpload > c.MaxPhotosPerLogbook {
		err = errors.Join(err, errors.New("MAX_PHOTOS_PER_UPLOAD cannot exceed MAX_PHOTOS_PER_LOGBOOK"))
	}
	if c.S3Endpoint != "" {
		u, e := url.Parse(c.S3Endpoint)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			err = errors.Join(err, errors.New("S3_ENDPOINT must be an http(s) endpoint URL"))
		} else if c.Environment == "production" && u.Scheme != "https" {
			err = errors.Join(err, errors.New("production S3_ENDPOINT requires HTTPS"))
		}
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		u, e := url.Parse(origin)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			err = errors.Join(err, errors.New("CORS_ALLOWED_ORIGINS must contain exact http(s) origins without a path"))
		} else {
			c.AllowedOrigins = append(c.AllowedOrigins, origin)
		}
	}
	return c, err
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
