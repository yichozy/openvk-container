package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestValidateDestURL(t *testing.T) {
	tests := []struct {
		dest string
		ok   bool
	}{
		{"rsync://replica:1935/viking/", true},
		{"rsync://10.0.0.1:873/data/", true},
		{"http://example.com/data", false},
		{"/local/path", false},
		{"rsync://", false},
		{"rsync:///viking/", false},
		{"rsync://host", false},
		{"", false},
	}
	for _, tc := range tests {
		err := validateDestURL(tc.dest)
		if tc.ok && err != nil {
			t.Errorf("expected ok for %q, got %v", tc.dest, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("expected error for %q", tc.dest)
		}
	}
}

func TestSyncViaHTTP(t *testing.T) {
	cfg := &Config{
		Port:         "0",
		Timeout:      30 * time.Second,
		SyncEnabled:  true,
		SyncSource:   "/nonexistent/src/",
		SyncDests:    []string{"rsync://replica:1935/viking/"},
		SyncExcludes: []string{"temp/"},
		SyncInterval: 10 * time.Minute,
	}
	syncer := NewSyncer(cfg)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, cfg, syncer)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// POST /sync — trigger sync (will fail because no rsync daemon, but flow is correct)
	resp, err := http.Post(srv.URL+"/sync", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(body))
	}

	// Poll /status until sync finishes
	deadline := time.After(10 * time.Second)
	for {
		resp, err := http.Get(srv.URL + "/status")
		if err != nil {
			t.Fatal(err)
		}
		var status StatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			t.Fatal(err)
		}
		resp.Body.Close()

		if !status.SyncState.Running {
			if len(status.SyncState.LastErrors) == 0 {
				t.Fatal("expected errors (no rsync daemon running)")
			}
			break
		}
		select {
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("timed out waiting for sync to complete")
		}
	}

	// POST /sync again while idle — should get 202
	resp2, err := http.Post(srv.URL+"/sync", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 on retry, got %d", resp2.StatusCode)
	}

	zap.L().Info("sync test passed", zap.Any("state", syncer.State()))
}
