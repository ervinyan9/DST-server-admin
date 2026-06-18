package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

func TestSupervisorStartAction(t *testing.T) {
	services := []serviceStatus{
		{Name: "dst-master", State: "running"},
		{Name: "dst-caves", State: "stopped"},
	}
	if got := supervisorStartAction("dst-master", services); got != "restart" {
		t.Fatalf("master action = %q, want restart", got)
	}
	if got := supervisorStartAction("dst-caves", services); got != "start" {
		t.Fatalf("caves action = %q, want start", got)
	}
	if got := supervisorStartAction("missing", services); got != "start" {
		t.Fatalf("missing action = %q, want start", got)
	}
}

func TestDSTServerBinaryPath(t *testing.T) {
	t.Setenv("DST_GAME_DIR", "/tmp/dst-game")
	want := "/tmp/dst-game/bin64/dontstarve_dedicated_server_nullrenderer_x64"
	if got := dstServerBinaryPath(); got != want {
		t.Fatalf("dstServerBinaryPath() = %q, want %q", got, want)
	}
}

func TestSearchLocalModsMatchesChineseDisplayTitle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	stateDir := filepath.Join(dir, "state")
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{
  "mods": [
    {
      "id": "2074508776",
      "title": "Auto Door",
      "display_title": "自动开门",
      "tags": ["server_only_mod"],
      "subscriptions": 169389,
      "time_updated": 1740157735
    },
    {
      "id": "3484720277",
      "title": "无限耐久",
      "display_title": "无限耐久",
      "tags": ["server_only_mod"],
      "subscriptions": 13,
      "time_updated": 1747356671
    },
    {
      "id": "111",
      "title": "Client Only",
      "display_title": "自动开门客户端",
      "tags": ["all_clients_require_mod"],
      "subscriptions": 999999,
      "time_updated": 1770000000
    }
  ]
}`
	stateData := `{
  "mods": [
    {
      "id": "2074508776",
      "title": "Auto Door",
      "display_title": "自动开门",
      "tags": ["server_only_mod"],
      "subscriptions": 169389,
      "time_updated": 1740157735
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "mods", "server-mods.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(stateDir, "server-mods.json")
	if err := os.WriteFile(stateFile, []byte(stateData), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{
		root:      root,
		stateFile: stateFile,
	}

	door := a.searchLocalMods("自动开门")
	if len(door) == 0 {
		t.Fatal("searchLocalMods returned no results for 自动开门")
	}
	if door[0].ID != "2074508776" {
		t.Fatalf("first 自动开门 result = %s, want 2074508776", door[0].ID)
	}
	if len(door) != 2 {
		t.Fatalf("自动开门 result count = %d, want 2 deduped installable results", len(door))
	}

	durability := a.searchLocalMods("无限耐久")
	if len(durability) == 0 {
		t.Fatal("searchLocalMods returned no results for 无限耐久")
	}
	if durability[0].ID != "3484720277" {
		t.Fatalf("first 无限耐久 result = %s, want 3484720277", durability[0].ID)
	}
}
