package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPasswordLoginReturnsDerivedAdminKey(t *testing.T) {
	a := &app{
		adminUsername: "admin",
		adminPassword: "ervindedst",
		adminKey:      deriveAdminKey("admin", "ervindedst"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", a.handleAuthLogin)
	mux.HandleFunc("/api/auth/verify", a.handleAuthVerify)
	server := httptest.NewServer(a.withAuth(mux))
	defer server.Close()

	unauthorized, err := http.Get(server.URL + "/api/auth/verify")
	if err != nil {
		t.Fatal(err)
	}
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("verify without key status = %d, want %d", unauthorized.StatusCode, http.StatusUnauthorized)
	}
	_ = unauthorized.Body.Close()

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"ervindedst"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		AdminKey string `json:"admin_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.AdminKey != a.adminKey {
		t.Fatal("login returned unexpected admin key")
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/auth/verify", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Admin-Key", body.AdminKey)
	verified, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer verified.Body.Close()
	if verified.StatusCode != http.StatusOK {
		t.Fatalf("verify with key status = %d, want %d", verified.StatusCode, http.StatusOK)
	}
}

func TestTailText(t *testing.T) {
	if got := tailText("abcdef", 3); got != "def" {
		t.Fatalf("tailText() = %q, want %q", got, "def")
	}
	if got := tailText("abc", 10); got != "abc" {
		t.Fatalf("tailText() = %q, want %q", got, "abc")
	}
}

func TestRunCommandOutputKillsProcessGroupOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runCommandOutput(ctx, exec.Command("sh", "-c", "sleep 5"))
	if err != context.DeadlineExceeded {
		t.Fatalf("runCommandOutput() error = %v, want %v", err, context.DeadlineExceeded)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("runCommandOutput() returned too late: %s", elapsed)
	}
}

func TestWorkshopDownloadTimeout(t *testing.T) {
	t.Setenv("DST_WORKSHOP_DOWNLOAD_TIMEOUT", "150ms")
	if got := workshopDownloadTimeout(); got != 150*time.Millisecond {
		t.Fatalf("workshopDownloadTimeout() = %s, want 150ms", got)
	}
}
