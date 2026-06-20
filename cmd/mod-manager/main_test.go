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

func TestDSTRuntimeLogsReturnsShardedTailMetadata(t *testing.T) {
	dir := t.TempDir()
	clusterDir := filepath.Join(dir, "cluster", "Cluster_1")
	masterDir := filepath.Join(clusterDir, "Master")
	cavesDir := filepath.Join(clusterDir, "Caves")
	if err := os.MkdirAll(masterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(masterDir, "server_log.txt"), []byte(strings.Repeat("A", 13000)+"MASTER-END"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cavesDir, "server_log.txt"), []byte("CAVES-END"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{dstDir: dir}

	logs, files := a.dstRuntimeLogs()
	if !strings.Contains(logs, "MASTER-END") || !strings.Contains(logs, "CAVES-END") {
		t.Fatalf("logs = %q, want shard tails", logs)
	}
	if len(files) != 3 {
		t.Fatalf("log file count = %d, want 3", len(files))
	}
	if !files[1].Truncated {
		t.Fatal("master log was not marked truncated")
	}
	if files[1].Size <= 12000 {
		t.Fatalf("master log size = %d, want > 12000", files[1].Size)
	}
	if files[1].UpdatedAt == "" {
		t.Fatal("master updated_at is empty")
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

func TestParseSupervisorStatusMapsStopping(t *testing.T) {
	services := parseSupervisorStatus("dst-master STOPPING\n")
	if len(services) != 1 {
		t.Fatalf("service count = %d, want 1", len(services))
	}
	if services[0].State != "stopping" {
		t.Fatalf("state = %q, want stopping", services[0].State)
	}
}

func TestDSTLogProblemMessage(t *testing.T) {
	if got := dstLogProblemMessage(`[200] Account Failed (6): "E_EXPIRED_TOKEN"`); !strings.Contains(got, "token") {
		t.Fatalf("expired token message = %q, want token hint", got)
	}
	if got := dstLogProblemMessage("DoLuaFile Could not load lua file scripts/main.lua"); !strings.Contains(got, "scripts/main.lua") {
		t.Fatalf("main.lua message = %q, want scripts/main.lua hint", got)
	}
	if got := dstLogProblemMessage("LOADING LUA SUCCESS"); got != "" {
		t.Fatalf("success log message = %q, want empty", got)
	}
}

func TestDSTServerBinaryPath(t *testing.T) {
	t.Setenv("DST_GAME_DIR", "/tmp/dst-game")
	want := "/tmp/dst-game/bin64/dontstarve_dedicated_server_nullrenderer_x64"
	if got := dstServerBinaryPath(); got != want {
		t.Fatalf("dstServerBinaryPath() = %q, want %q", got, want)
	}
}

func TestGenerateSettingsFilesPreservesCavesShardID(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "data")
	clusterDir := filepath.Join(dstDir, "cluster", "Cluster_1")
	cavesDir := filepath.Join(clusterDir, "Caves")
	if err := os.MkdirAll(cavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cavesPath := filepath.Join(cavesDir, "server.ini")
	if err := os.WriteFile(cavesPath, []byte("[SHARD]\nis_master = false\nname = Caves\nid = 2407032924\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{
		dstDir:       dstDir,
		settingsFile: filepath.Join(dstDir, "admin", "server-settings.json"),
	}
	if _, err := a.generateSettingsFiles(serverSettings{ServerName: "EvanDST", MaxPlayers: 6, EnableCaves: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cavesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "id = 2407032924") {
		t.Fatalf("caves server.ini = %q, want preserved shard id", string(data))
	}
}

func TestGenerateSettingsFilesDoesNotInventCavesShardID(t *testing.T) {
	dir := t.TempDir()
	a := &app{
		dstDir:       filepath.Join(dir, "data"),
		settingsFile: filepath.Join(dir, "data", "admin", "server-settings.json"),
	}
	if _, err := a.generateSettingsFiles(serverSettings{ServerName: "EvanDST", MaxPlayers: 6, EnableCaves: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "data", "cluster", "Cluster_1", "Caves", "server.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "id =") {
		t.Fatalf("caves server.ini = %q, want no generated shard id", string(data))
	}
}

func TestRecordModDownloadStatusPersistsState(t *testing.T) {
	dir := t.TempDir()
	a := &app{
		stateFile: filepath.Join(dir, "server-mods.json"),
	}
	if err := a.saveState(state{Mods: []mod{{ID: "123", Title: "Test", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := a.recordModDownloadStatus("123", "error", "download failed", ""); err != nil {
		t.Fatal(err)
	}
	s, err := a.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Mods) != 1 {
		t.Fatalf("mod count = %d, want 1", len(s.Mods))
	}
	if s.Mods[0].DownloadStatus != "error" || s.Mods[0].DownloadMessage != "download failed" {
		t.Fatalf("download status = %+v, want persisted error", s.Mods[0])
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

func TestSQLiteStoreImportsLegacyStateAndRestoresToken(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "admin")
	dstDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	clusterDir := filepath.Join(dstDir, "cluster", "Cluster_1")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(stateDir, "server-mods.json")
	settingsFile := filepath.Join(stateDir, "server-settings.json")
	if err := os.WriteFile(stateFile, []byte(`{"mods":[{"id":"2074508776","title":"Auto Door","tags":["server_only_mod"],"enabled":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsFile, []byte(`{"server_name":"EvanDST","game_mode":"endless","max_players":8,"pause_when_empty":true,"enable_caves":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(clusterDir, "cluster_token.txt")
	if err := os.WriteFile(tokenPath, []byte("pds-test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{
		stateFile:    stateFile,
		settingsFile: settingsFile,
		dstDir:       dstDir,
		dbPath:       filepath.Join(stateDir, "dst-admin.db"),
	}
	if err := a.ensureState(); err != nil {
		t.Fatal(err)
	}
	defer a.db.Close()

	loaded, err := a.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Settings.ServerName != "EvanDST" || loaded.Settings.GameMode != "endless" {
		t.Fatalf("settings = %+v, want imported settings", loaded.Settings)
	}
	if len(loaded.Mods) != 1 || loaded.Mods[0].ID != "2074508776" {
		t.Fatalf("mods = %+v, want imported mod", loaded.Mods)
	}
	token, err := a.loadClusterTokenPlaintext()
	if err != nil {
		t.Fatal(err)
	}
	if token != "pds-test-token" {
		t.Fatalf("stored token = %q, want original token", token)
	}

	if err := os.Remove(tokenPath); err != nil {
		t.Fatal(err)
	}
	if err := a.restoreClusterTokenFile(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(restored)) != "pds-test-token" {
		t.Fatalf("restored token = %q, want original token", strings.TrimSpace(string(restored)))
	}
}

func TestTokenLogProblemIgnoresLogsOlderThanSavedToken(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "data")
	clusterDir := filepath.Join(dstDir, "cluster", "Cluster_1", "Master")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(clusterDir, "server_log.txt")
	if err := os.WriteFile(logPath, []byte(`[200] Account Failed (6): "E_EXPIRED_TOKEN"`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(logPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	a := &app{
		dstDir: dstDir,
		dbPath: filepath.Join(dir, "admin", "dst-admin.db"),
	}
	if err := a.ensureStore(); err != nil {
		t.Fatal(err)
	}
	defer a.db.Close()
	if err := a.saveClusterTokenToStore("pds-new-token"); err != nil {
		t.Fatal(err)
	}
	if a.tokenLogProblemIsCurrent(a.currentTokenStatus()) {
		t.Fatal("old token error log was treated as current")
	}

	newTime := time.Now().Add(time.Hour)
	if err := os.Chtimes(logPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	if !a.tokenLogProblemIsCurrent(a.currentTokenStatus()) {
		t.Fatal("new token error log was not treated as current")
	}
}
