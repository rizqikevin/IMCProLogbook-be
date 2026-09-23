package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontendAndProxy(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<html>app</html>", "assets/app.js": "app()"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "logbook.example.com" || r.Header.Get("Authorization") != "Bearer example" {
			t.Error("lost public host or authorization")
		}
		if r.URL.RawQuery != "page=2" {
			t.Error("lost query")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "photo-body" {
			t.Error("lost upload body")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("saved"))
	}))
	defer backend.Close()
	upstream, _ := url.Parse(backend.URL)
	h := handler(root, upstream)
	for _, tc := range []struct {
		path        string
		status      int
		body, cache string
	}{
		{"/login", 200, "<html>app</html>", "no-cache"},
		{"/logbooks/123/add", 200, "<html>app</html>", "no-cache"},
		{"/assets/app.js", 200, "app()", "public, max-age=31536000, immutable"},
		{"/assets/missing.js", 404, "", ""},
		{"/missing.js", 404, "", ""},
		{"/health", 200, "", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status %d", w.Code)
			}
			if tc.body != "" && w.Body.String() != tc.body {
				t.Fatalf("body %q", w.Body.String())
			}
			if tc.cache != "" && w.Header().Get("Cache-Control") != tc.cache {
				t.Fatal("wrong cache policy")
			}
			if w.Header().Get("Content-Security-Policy") == "" {
				t.Fatal("missing CSP")
			}
		})
	}
	req := httptest.NewRequest("POST", "http://logbook.example.com/api/v1/logbooks?page=2", strings.NewReader("photo-body"))
	req.Header.Set("Authorization", "Bearer example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 || w.Body.String() != "saved" {
		t.Fatalf("proxy: %d %s", w.Code, w.Body.String())
	}
}
