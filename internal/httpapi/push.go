package httpapi

import (
	"encoding/json"
	"machine-logbook/internal/domain"
	"net/http"
)

func (s *Server) pushConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.opts.Push != nil && s.opts.PushPublicKey != "", "public_key": s.opts.PushPublicKey})
}

func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.opts.Push == nil {
		problem(w, r, http.StatusNotFound, "push_disabled", "push is disabled")
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256DH string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		problem(w, r, 400, "invalid_request", "invalid subscription")
		return
	}
	user := currentUser(r)
	if err := s.opts.Push.Subscribe(r.Context(), domain.PushSubscription{UserID: user.ID, Endpoint: body.Endpoint, P256DH: body.Keys.P256DH, Auth: body.Keys.Auth}); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if s.opts.Push == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil || body.Endpoint == "" {
		problem(w, r, 400, "invalid_request", "endpoint is required")
		return
	}
	if err := s.opts.Push.Revoke(r.Context(), currentUser(r).ID, body.Endpoint); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
