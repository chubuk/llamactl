package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func cfgForServer(t *testing.T, srv *httptest.Server) config {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return config{port: port}
}

func TestWaitReady(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	if err := waitReady(cfgForServer(t, ok), 2*time.Second); err != nil {
		t.Errorf("waitReady(ok server) = %v, want nil", err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	cfg := cfgForServer(t, down)
	down.Close()
	if err := waitReady(cfg, 3*time.Second); err == nil {
		t.Error("waitReady(down server) = nil, want error")
	}
}

func TestProbeMode(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		wantOn bool
		wantOK bool
	}{
		{"enabled", `{"cors_proxy_enabled": true}`, true, true},
		{"disabled", `{"cors_proxy_enabled": false}`, false, true},
		{"not json", `Internal Server Error`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/props" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			on, ok := probeMode(cfgForServer(t, srv))
			if ok != tt.wantOK || on != tt.wantOn {
				t.Errorf("probeMode() = (%v, %v), want (%v, %v)", on, ok, tt.wantOn, tt.wantOK)
			}
		})
	}
}
