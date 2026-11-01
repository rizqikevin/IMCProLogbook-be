package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"machine-logbook/internal/domain"
)

type PushRepository interface {
	UpsertPushSubscription(context.Context, domain.PushSubscription) error
	PushSubscriptions(context.Context, string) ([]domain.PushSubscription, error)
	RevokePushSubscription(context.Context, string, string) error
	DeletePushSubscription(context.Context, string) error
}

type PushService struct {
	repo         PushRepository
	options      webpush.Options
	jobs         chan pushJob
	trustedHosts []string
	stop         chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}
type pushJob struct {
	userID  string
	payload []byte
}
type PushConfig struct {
	VAPIDPublicKey, VAPIDPrivateKey, Subscriber string
	Workers, Queue                              int
	TrustedHosts                                []string
}

func NewPushService(repo PushRepository, cfg PushConfig) *PushService {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.Queue < 1 {
		cfg.Queue = 64
	}
	p := &PushService{repo: repo, trustedHosts: cfg.TrustedHosts, jobs: make(chan pushJob, cfg.Queue), stop: make(chan struct{}), options: webpush.Options{VAPIDPublicKey: cfg.VAPIDPublicKey, VAPIDPrivateKey: cfg.VAPIDPrivateKey, Subscriber: cfg.Subscriber, TTL: 60, HTTPClient: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}
	p.wg.Add(cfg.Workers)
	for range cfg.Workers {
		go p.worker()
	}
	return p
}

func (p *PushService) Close() { p.closeOnce.Do(func() { close(p.stop) }); p.wg.Wait() }

func (p *PushService) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.stop:
			return
		case j := <-p.jobs:
			subs, err := p.repo.PushSubscriptions(context.Background(), j.userID)
			if err != nil {
				slog.Error("push subscription lookup failed", "error_type", fmt.Sprintf("%T", err))
				continue
			}
			for _, s := range subs {
				provider := ""
				if endpoint, parseErr := url.Parse(s.Endpoint); parseErr == nil {
					provider = endpoint.Hostname()
				}
				if !validPushEndpoint(s.Endpoint, p.trustedHosts) {
					slog.Warn("push endpoint rejected", "provider", provider)
					continue
				}
				resp, err := webpush.SendNotification(j.payload, &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256DH, Auth: s.Auth}}, &p.options)
				if err != nil {
					// Network errors may include the endpoint's secret subscription token.
					slog.Error("push delivery failed", "provider", provider, "error_type", fmt.Sprintf("%T", err))
					continue
				}
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					slog.Warn("push provider rejected delivery", "provider", provider, "status", resp.StatusCode)
				}
				if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
					if err := p.repo.DeletePushSubscription(context.Background(), s.Endpoint); err != nil {
						slog.Error("push subscription cleanup failed", "provider", provider, "status", resp.StatusCode, "error_type", fmt.Sprintf("%T", err))
					}
				}
			}
		}
	}
}

func (p *PushService) Enqueue(userID string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	select {
	case <-p.stop:
		return fmt.Errorf("push service closed")
	default:
	}
	select {
	case p.jobs <- pushJob{userID: userID, payload: b}:
		return nil
	case <-p.stop:
		return fmt.Errorf("push service closed")
	}
}

func (p *PushService) Subscribe(ctx context.Context, s domain.PushSubscription) error {
	if s.UserID == "" || !validPushEndpoint(s.Endpoint, p.trustedHosts) || len(s.P256DH) < 40 || len(s.Auth) < 16 {
		return &domain.ValidationError{Message: "invalid push subscription"}
	}
	return p.repo.UpsertPushSubscription(ctx, s)
}

func (p *PushService) EnqueueUsers(userIDs []string, excludeUserID string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for _, userID := range userIDs {
		if userID == "" || userID == excludeUserID {
			continue
		}
		select {
		case <-p.stop:
			return fmt.Errorf("push service closed")
		default:
		}
		select {
		case p.jobs <- pushJob{userID: userID, payload: b}:
		default:
			return fmt.Errorf("push queue full")
		}
	}
	return nil
}

func (p *PushService) Revoke(ctx context.Context, userID, endpoint string) error {
	if userID == "" || endpoint == "" {
		return &domain.ValidationError{Message: "endpoint is required"}
	}
	return p.repo.RevokePushSubscription(ctx, userID, endpoint)
}

func validPushEndpoint(raw string, trusted []string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	for _, host := range trusted {
		if strings.EqualFold(u.Hostname(), host) {
			return true
		}
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return false
		}
	}
	return true
}
