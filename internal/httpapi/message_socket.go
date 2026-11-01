package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"machine-logbook/internal/domain"
)

type socketCommand struct {
	Type      string `json:"type"`
	Token     string `json:"token,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Text      string `json:"text,omitempty"`
}

func (s *Server) messageSocket(w http.ResponseWriter, r *http.Request) {
	machineID, err := strconv.ParseInt(r.PathValue("machineID"), 10, 64)
	if err != nil || machineID <= 0 || r.URL.RawQuery != "" {
		problem(w, r, 400, "validation_error", "invalid machine or query")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.opts.AllowedOrigins})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(128 << 10)
	parent := s.opts.SocketContext
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	write := func(value any) error {
		writeCtx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		return wsjson.Write(writeCtx, conn, value)
	}
	authCtx, done := context.WithTimeout(ctx, 5*time.Second)
	var first socketCommand
	err = wsjson.Read(authCtx, conn, &first)
	if err == nil && first.Type == "auth" {
		if first.Token == "" {
			first.Token, _ = sessionToken(r)
			if !s.trustedRequestOrigin(r) {
				err = domain.ErrUnauthorized
			}
		}
		if err == nil {
			_, err = s.auth.Authenticate(authCtx, first.Token)
		}
	} else if err == nil {
		err = domain.ErrUnauthorized
	}
	done()
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	checkCtx, done := context.WithTimeout(ctx, 10*time.Second)
	_, err = s.logbooks.ListMessages(checkCtx, domain.MessageFilter{MachineID: machineID, Limit: 1})
	done()
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "machine unavailable")
		return
	}
	events, unsubscribe := s.logbooks.SubscribeMessages(machineID)
	defer unsubscribe()
	if write(map[string]string{"type": "ready"}) != nil {
		return
	}
	commands := make(chan socketCommand)
	go func() {
		defer cancel()
		for {
			var command socketCommand
			if wsjson.Read(ctx, conn, &command) != nil {
				return
			}
			select {
			case commands <- command:
			case <-ctx.Done():
				return
			}
		}
	}()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			authCtx, done := context.WithTimeout(ctx, 10*time.Second)
			_, err := s.auth.Authenticate(authCtx, first.Token)
			done()
			if err != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
				return
			}
			if write(event) != nil {
				return
			}
		case <-ticker.C:
			pingCtx, done := context.WithTimeout(ctx, 10*time.Second)
			_, err := s.auth.Authenticate(pingCtx, first.Token)
			if err == nil {
				err = conn.Ping(pingCtx)
			}
			done()
			if err != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "session or connection ended")
				return
			}
		case command := <-commands:
			opCtx, done := context.WithTimeout(ctx, s.opts.RequestTimeout)
			user, err := s.auth.Authenticate(opCtx, first.Token)
			if err != nil {
				done()
				_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
				return
			}
			if command.Type != "send" || command.RequestID == "" || len(command.RequestID) > 128 {
				done()
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid command")
				return
			}
			message, err := s.logbooks.CreateMessage(opCtx, user, machineID, command.Text, nil)
			done()
			if err != nil {
				code, detail := "internal_error", "Server belum dapat mengirim pesan. Periksa riwayat sebelum mencoba lagi."
				var validation *domain.ValidationError
				if errors.As(err, &validation) {
					code, detail = "validation_error", validation.Message
				}
				if write(map[string]any{"type": "error", "request_id": command.RequestID, "error": map[string]string{"code": code, "message": detail}}) != nil {
					return
				}
				continue
			}
			if write(map[string]any{"type": "ack", "request_id": command.RequestID, "message": message}) != nil {
				return
			}
		}
	}
}
