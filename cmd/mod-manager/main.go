package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const appID = "322330"

type app struct {
	root           string
	stateFile      string
	settingsFile   string
	dstDir         string
	supervisorConf string
	adminKey       string
	adminUsername  string
	adminPassword  string
	assetVersion   string
	client         *http.Client
}

type state struct {
	Mods     []mod          `json:"mods"`
	Settings serverSettings `json:"settings"`
}

type serverSettings struct {
	ServerName     string `json:"server_name"`
	ServerPassword string `json:"server_password"`
	GameMode       string `json:"game_mode"`
	MaxPlayers     int    `json:"max_players"`
	PVP            bool   `json:"pvp"`
	PauseWhenEmpty bool   `json:"pause_when_empty"`
	EnableCaves    bool   `json:"enable_caves"`
}

type mod struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	DisplayTitle   string   `json:"display_title"`
	Category       string   `json:"category"`
	Description    string   `json:"description"`
	Preview        string   `json:"preview"`
	FileURL        string   `json:"file_url"`
	Subscriptions  int64    `json:"subscriptions"`
	TimeUpdated    int64    `json:"time_updated"`
	Tags           []string `json:"tags"`
	ServerOnly     bool     `json:"server_only"`
	ClientRequired bool     `json:"client_required"`
	Installable    bool     `json:"installable"`
	BlockedReason  string   `json:"blocked_reason,omitempty"`
	Enabled        bool     `json:"enabled"`
}

type playerInfo struct {
	KUID     string `json:"kuid"`
	Name     string `json:"name"`
	LastSeen string `json:"last_seen"`
	Admin    bool   `json:"admin"`
}

type serverStatusResponse struct {
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	CheckedAt string          `json:"checked_at"`
	Services  []serviceStatus `json:"services"`
	Logs      string          `json:"logs"`
}

type searchResponse struct {
	Results  []mod  `json:"results"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Sort     string `json:"sort"`
	Query    string `json:"query"`
	HasMore  bool   `json:"has_more"`
}

type modDiagnostic struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	DisplayTitle       string   `json:"display_title"`
	Enabled            bool     `json:"enabled"`
	ConfiguredSetup    bool     `json:"configured_setup"`
	ConfiguredOverride bool     `json:"configured_override"`
	Downloaded         bool     `json:"downloaded"`
	LegacyPackage      bool     `json:"legacy_package"`
	LoadedInLogs       bool     `json:"loaded_in_logs"`
	Outdated           bool     `json:"outdated"`
	Paths              []string `json:"paths"`
	Problems           []string `json:"problems"`
	UpdatedAt          string   `json:"updated_at"`
}

type modDownloadResponse struct {
	Status     string        `json:"status"`
	Message    string        `json:"message"`
	ID         string        `json:"id"`
	Output     string        `json:"output"`
	Diagnostic modDiagnostic `json:"diagnostic"`
}

type serviceStatus struct {
	Name         string `json:"name"`
	Service      string `json:"service"`
	State        string `json:"state"`
	Health       string `json:"health"`
	Status       string `json:"status"`
	StartedAt    string `json:"started_at"`
	RunningFor   string `json:"running_for"`
	RestartCount string `json:"restart_count"`
}

type detailsResponse struct {
	Response struct {
		Details []struct {
			PublishedFileID string `json:"publishedfileid"`
			Title           string `json:"title"`
			Description     string `json:"description"`
			PreviewURL      string `json:"preview_url"`
			Subscriptions   int64  `json:"subscriptions"`
			TimeUpdated     int64  `json:"time_updated"`
			Tags            []struct {
				Tag string `json:"tag"`
			} `json:"tags"`
		} `json:"publishedfiledetails"`
	} `json:"response"`
}

var idPattern = regexp.MustCompile(`sharedfiles/filedetails/\?id=(\d+)`)
var numericPattern = regexp.MustCompile(`^\d+$`)
var kuidPattern = regexp.MustCompile(`KU_[A-Za-z0-9]+`)
var kuidExactPattern = regexp.MustCompile(`^KU_[A-Za-z0-9]+$`)
var quotedNamePattern = regexp.MustCompile(`"([^"]{1,64})"`)
var namedFieldPattern = regexp.MustCompile(`(?i)(?:name|player|username)\s*[:=]\s*['"]?([^,'"\]\)]+)`)
var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)

func main() {
	port := flag.Int("port", 8788, "HTTP port")
	listen := flag.String("listen", "127.0.0.1", "HTTP listen address")
	root := flag.String("root", "/opt/dst/admin", "admin root directory (templates, static assets)")
	dstDir := flag.String("dst-dir", "/data", "DST persistent data root (cluster/mods/ugc_mods/admin live here)")
	supervisorConf := flag.String("supervisor-conf", "/opt/dst/runtime/supervisord.conf", "supervisord config used for supervisorctl")
	adminKey := flag.String("admin-key", os.Getenv("DST_ADMIN_KEY"), "admin key for API requests")
	adminUsername := flag.String("admin-username", os.Getenv("DST_ADMIN_USERNAME"), "admin login username")
	adminPassword := flag.String("admin-password", os.Getenv("DST_ADMIN_PASSWORD"), "admin login password")
	flag.Parse()

	absRoot, err := resolveRoot(*root)
	if err != nil {
		log.Fatal(err)
	}

	effectiveAdminKey := *adminKey
	if *adminUsername != "" && *adminPassword != "" {
		effectiveAdminKey = deriveAdminKey(*adminUsername, *adminPassword)
	}

	adminStateDir := filepath.Join(*dstDir, "admin")
	a := &app{
		root:           absRoot,
		stateFile:      filepath.Join(adminStateDir, "server-mods.json"),
		settingsFile:   filepath.Join(adminStateDir, "server-settings.json"),
		dstDir:         *dstDir,
		supervisorConf: *supervisorConf,
		adminKey:       effectiveAdminKey,
		adminUsername:  *adminUsername,
		adminPassword:  *adminPassword,
		assetVersion:   strconv.FormatInt(time.Now().Unix(), 10),
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
	if err := a.ensureState(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	staticFiles := http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(absRoot, "web", "static"))))
	mux.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		staticFiles.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/api/auth/login", a.handleAuthLogin)
	mux.HandleFunc("/api/auth/verify", a.handleAuthVerify)
	mux.HandleFunc("/api/state", a.handleState)
	mux.HandleFunc("/api/search", a.handleSearch)
	mux.HandleFunc("/api/popular", a.handlePopular)
	mux.HandleFunc("/api/mods", a.handleAddMod)
	mux.HandleFunc("/api/mods/toggle", a.handleToggleMod)
	mux.HandleFunc("/api/mods/remove", a.handleRemoveMod)
	mux.HandleFunc("/api/mods/diagnostics", a.handleModDiagnostics)
	mux.HandleFunc("/api/mods/download", a.handleModDownload)
	mux.HandleFunc("/api/settings", a.handleSettings)
	mux.HandleFunc("/api/cluster-token", a.handleClusterToken)
	mux.HandleFunc("/api/restart", a.handleRestartServer)
	mux.HandleFunc("/api/server/status", a.handleServerStatus)
	mux.HandleFunc("/api/players", a.handlePlayers)
	mux.HandleFunc("/api/players/admin/add", a.handleAdminAdd)
	mux.HandleFunc("/api/players/admin/remove", a.handleAdminRemove)

	addr := fmt.Sprintf("%s:%d", *listen, *port)
	if a.adminKey == "" {
		log.Print("WARNING: admin key is empty; API is not protected")
	} else if !a.passwordLoginEnabled() {
		log.Print("WARNING: username/password login is disabled; using legacy admin key auth")
	}
	log.Printf("DST MOD manager listening on http://%s/", addr)
	log.Fatal(http.ListenAndServe(addr, a.withAuth(mux)))
}

func (a *app) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.adminKey != "" && strings.HasPrefix(r.URL.Path, "/api/") {
			if r.URL.Path == "/api/auth/login" {
				next.ServeHTTP(w, r)
				return
			}
			got := r.Header.Get("X-Admin-Key")
			if got == "" {
				got = r.URL.Query().Get("key")
			}
			if got != a.adminKey {
				http.Error(w, "需要管理密钥", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func deriveAdminKey(username string, password string) string {
	sum := sha256.Sum256([]byte("dst-waystone-admin-key:v1\x00" + username + "\x00" + password))
	return hex.EncodeToString(sum[:])
}

func (a *app) passwordLoginEnabled() bool {
	return a.adminUsername != "" && a.adminPassword != ""
}

func constantTimeEqualString(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func resolveRoot(input string) (string, error) {
	absRoot, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	if hasProjectFiles(absRoot) {
		return absRoot, nil
	}
	if input != "." {
		return "", fmt.Errorf("project root %q does not contain web/templates/index.html", absRoot)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if hasProjectFiles(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	exe, err := os.Executable()
	if err == nil {
		dir = filepath.Dir(exe)
		for {
			if hasProjectFiles(dir) {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return "", fmt.Errorf("cannot find project root; run with -root D:\\Projects\\DST")
}

func hasProjectFiles(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "web", "templates", "index.html"))
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "web", "static", "app.js"))
	return err == nil
}

func (a *app) ensureState() error {
	if err := os.MkdirAll(filepath.Dir(a.stateFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.settingsFile), 0o755); err != nil {
		return err
	}
	clusterDir := filepath.Join(a.dstDir, "cluster", "Cluster_1")
	for _, sub := range []string{"Master", "Caves", "mods"} {
		if err := os.MkdirAll(filepath.Join(clusterDir, sub), 0o755); err != nil {
			return err
		}
	}
	for _, dir := range []string{"mods", "ugc_mods"} {
		if err := os.MkdirAll(filepath.Join(a.dstDir, dir), 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(a.stateFile); os.IsNotExist(err) {
		return a.saveState(state{Mods: []mod{}})
	}
	if _, err := os.Stat(a.settingsFile); os.IsNotExist(err) {
		if s, loadErr := a.loadStateFromMainFile(); loadErr == nil {
			return a.saveSettingsCache(s.Settings)
		}
	}
	return nil
}

func (a *app) loadState() (state, error) {
	s, err := a.loadStateFromMainFile()
	if err != nil {
		return s, err
	}
	settings, err := a.loadSettings(s.Settings)
	if err != nil {
		return s, err
	}
	s.Settings = settings
	return s, nil
}

func (a *app) loadStateFromMainFile() (state, error) {
	var s state
	data, err := os.ReadFile(a.stateFile)
	if err != nil {
		return s, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(data)) == 0 {
		return state{Mods: []mod{}}, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Mods == nil {
		s.Mods = []mod{}
	}
	for i := range s.Mods {
		s.Mods[i].ServerOnly = hasTag(s.Mods[i].Tags, "server_only_mod")
		s.Mods[i].ClientRequired = hasTag(s.Mods[i].Tags, "all_clients_require_mod")
		s.Mods[i].Installable = s.Mods[i].ServerOnly || s.Mods[i].ClientRequired
	}
	s.Mods = localizeMods(s.Mods)
	s.Settings = normalizeSettings(s.Settings)
	return s, nil
}

func (a *app) loadSettings(fallback serverSettings) (serverSettings, error) {
	data, err := os.ReadFile(a.settingsFile)
	if err == nil {
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		if len(bytes.TrimSpace(data)) > 0 {
			var settings serverSettings
			if err := json.Unmarshal(data, &settings); err != nil {
				return serverSettings{}, err
			}
			return normalizeSettings(settings), nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return serverSettings{}, err
	}
	return normalizeSettings(fallback), nil
}

func (a *app) saveSettingsCache(settings serverSettings) error {
	settings = normalizeSettings(settings)
	if err := os.MkdirAll(filepath.Dir(a.settingsFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.settingsFile, append(data, '\n'), 0o644)
}

func normalizeSettings(input serverSettings) serverSettings {
	if input.ServerName == "" && input.ServerPassword == "" && input.GameMode == "" && input.MaxPlayers == 0 {
		return defaultSettings()
	}
	input.GameMode = normalizeGameMode(input.GameMode)
	if input.ServerName == "" {
		input.ServerName = defaultSettings().ServerName
	}
	if input.MaxPlayers <= 0 {
		input.MaxPlayers = defaultSettings().MaxPlayers
	}
	return input
}

func defaultSettings() serverSettings {
	return serverSettings{
		ServerName:     "DST Waystone",
		ServerPassword: "",
		GameMode:       "survival",
		MaxPlayers:     6,
		PVP:            false,
		PauseWhenEmpty: true,
		EnableCaves:    true,
	}
}

func normalizeGameMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "endless", "无尽", "endless mode":
		return "endless"
	default:
		return "survival"
	}
}

func (a *app) saveState(s state) error {
	if cached, err := a.loadSettings(s.Settings); err == nil {
		s.Settings = cached
	} else {
		s.Settings = normalizeSettings(s.Settings)
	}
	sort.SliceStable(s.Mods, func(i, j int) bool {
		return strings.ToLower(s.Mods[i].Title) < strings.ToLower(s.Mods[j].Title)
	})
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.stateFile, append(data, '\n'), 0o644)
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles(filepath.Join(a.root, "web", "templates", "index.html"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	_ = tmpl.Execute(w, map[string]string{
		"AppID":        appID,
		"AssetVersion": a.assetVersion,
	})
}

func (a *app) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (a *app) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.passwordLoginEnabled() {
		http.Error(w, "未配置管理端用户名和密码", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(req.Username)
	if !constantTimeEqualString(username, a.adminUsername) || !constantTimeEqualString(req.Password, a.adminPassword) {
		http.Error(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{
		"admin_key": a.adminKey,
		"status":    "ok",
	})
}

func (a *app) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{
		"ok":      true,
		"message": "授权成功",
	})
}

func (a *app) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parseBoundedInt(r.URL.Query().Get("page"), 1, 1, 20)
	pageSize := parseBoundedInt(r.URL.Query().Get("page_size"), 24, 1, 50)
	sortName := normalizeWorkshopSort(r.URL.Query().Get("sort"))
	if query == "" {
		writeJSON(w, searchResponse{Results: []mod{}, Page: page, PageSize: pageSize, Sort: sortName, Query: query})
		return
	}
	if numericPattern.MatchString(query) {
		results, err := a.searchWorkshopID(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, searchResponse{Results: results, Page: 1, PageSize: pageSize, Sort: sortName, Query: query, HasMore: false})
		return
	}
	results, hasMore, err := a.searchWorkshopPaged(query, sortName, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, searchResponse{
		Results:  results,
		Page:     page,
		PageSize: pageSize,
		Sort:     sortName,
		Query:    query,
		HasMore:  hasMore,
	})
}

func (a *app) searchWorkshopID(id string) ([]mod, error) {
	items, err := a.getWorkshopDetails([]string{id})
	if err != nil {
		return nil, err
	}
	items = localizeMods(items)
	for i := range items {
		if !items[i].Installable {
			items[i].BlockedReason = "不是服务端可安装 MOD，不能安装到专用服务器"
		}
	}
	return items, nil
}

func (a *app) handlePopular(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 40
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}
	results, err := a.popularServerMods(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string][]mod{"results": results})
}

func (a *app) handleAddMod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if !numericPattern.MatchString(req.ID) {
		http.Error(w, "Workshop ID must be numeric", http.StatusBadRequest)
		return
	}
	items, err := a.getWorkshopDetails([]string{req.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	item := mod{
		ID:      req.ID,
		Title:   "Workshop " + req.ID,
		FileURL: "https://steamcommunity.com/sharedfiles/filedetails/?id=" + req.ID,
		Enabled: true,
	}
	if len(items) > 0 {
		item = items[0]
		item.Enabled = true
	}
	if !item.Installable {
		http.Error(w, "这个 MOD 不是服务端可安装类型", http.StatusBadRequest)
		return
	}

	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next := s.Mods[:0]
	for _, existing := range s.Mods {
		if existing.ID != req.ID {
			next = append(next, existing)
		}
	}
	s.Mods = append(next, item)
	if err := a.saveState(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := a.applyConfig(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (a *app) handleToggleMod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range s.Mods {
		if s.Mods[i].ID == req.ID {
			s.Mods[i].Enabled = req.Enabled
		}
	}
	if err := a.saveState(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := a.applyConfig(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (a *app) handleRemoveMod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next := make([]mod, 0, len(s.Mods))
	for _, existing := range s.Mods {
		if existing.ID != req.ID {
			next = append(next, existing)
		}
	}
	s.Mods = next
	if err := a.saveState(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := a.applyConfig(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (a *app) handleModDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	diagnostics, err := a.modDiagnostics()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"diagnostics": diagnostics,
		"checked_at":  time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (a *app) handleModDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if !numericPattern.MatchString(req.ID) {
		http.Error(w, "Workshop ID 必须是数字", http.StatusBadRequest)
		return
	}
	result, err := a.downloadWorkshopMod(req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (a *app) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req serverSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req = normalizeSettings(req)
	if req.MaxPlayers < 1 || req.MaxPlayers > 64 {
		http.Error(w, "最大人数必须在 1 到 64 之间", http.StatusBadRequest)
		return
	}
	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.Settings = req
	if err := a.saveSettingsCache(req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.saveState(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	applied, err := a.applyConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"status":  "ok",
		"state":   s,
		"applied": applied,
	})
}

func (a *app) handleRestartServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.clusterTokenPresent() {
		http.Error(w, "未配置 Klei cluster token，无法启动 DST 服务", http.StatusBadRequest)
		return
	}
	if err := a.ensureDSTServerBinary(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, err := a.loadState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	results := []map[string]string{}
	services, statusErr := a.currentSupervisorServices()
	if statusErr != nil {
		http.Error(w, fmt.Sprintf("读取 supervisor 状态失败：%v", statusErr), http.StatusInternalServerError)
		return
	}
	masterAction := supervisorStartAction("dst-master", services)
	masterOut, masterErr := a.supervisorctl(masterAction, "dst-master")
	results = append(results, map[string]string{"action": masterAction + " dst-master", "output": masterOut, "error": errString(masterErr)})
	if s.Settings.EnableCaves {
		cavesAction := supervisorStartAction("dst-caves", services)
		out, err := a.supervisorctl(cavesAction, "dst-caves")
		results = append(results, map[string]string{"action": cavesAction + " dst-caves", "output": out, "error": errString(err)})
	} else {
		out, err := a.supervisorctl("stop", "dst-caves")
		results = append(results, map[string]string{"action": "stop dst-caves", "output": out, "error": errString(err)})
	}
	if masterErr != nil {
		http.Error(w, fmt.Sprintf("重启失败：%v\n%s", masterErr, masterOut), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"status":  "ok",
		"actions": results,
	})
}

func (a *app) handleClusterToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "token 不能为空", http.StatusBadRequest)
		return
	}
	clusterDir, err := a.clusterDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tokenPath := filepath.Join(clusterDir, "cluster_token.txt")
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{
		"status": "ok",
		"path":   tokenPath,
	})
}

func (a *app) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := a.serverStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, status)
}

func (a *app) handlePlayers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	players, err := a.loadPlayersFromLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string][]playerInfo{"players": players})
}

func (a *app) handleAdminAdd(w http.ResponseWriter, r *http.Request) {
	a.handleAdminChange(w, r, true)
}

func (a *app) handleAdminRemove(w http.ResponseWriter, r *http.Request) {
	a.handleAdminChange(w, r, false)
}

func (a *app) handleAdminChange(w http.ResponseWriter, r *http.Request, add bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		KUID string `json:"kuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.KUID = strings.TrimSpace(req.KUID)
	if !kuidExactPattern.MatchString(req.KUID) {
		http.Error(w, "KUID 格式不正确", http.StatusBadRequest)
		return
	}
	admins, err := a.readAdminSet()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if add {
		admins[req.KUID] = true
	} else {
		delete(admins, req.KUID)
	}
	if err := a.writeAdminSet(admins); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	players, err := a.loadPlayersFromLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"status":  "ok",
		"players": players,
		"message": "管理员列表已保存，重启服务器后生效",
	})
}

func (a *app) popularServerMods(limit int) ([]mod, error) {
	sorts := []string{"timeupdated", "trend", "totaluniquesubscribers", "toprated"}
	seen := map[string]bool{}
	var ids []string

	for _, sortName := range sorts {
		pageIDs, err := a.scrapeWorkshopIDs("", sortName, 2)
		if err != nil {
			log.Printf("popular scrape failed for sort %s: %v", sortName, err)
			continue
		}
		for _, id := range pageIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("failed to fetch popular Workshop list")
	}

	items, err := a.getWorkshopDetails(ids)
	if err != nil {
		return nil, err
	}
	serverOnly := filterServerOnly(items, limit)
	sort.SliceStable(serverOnly, func(i, j int) bool {
		if serverOnly[i].TimeUpdated != serverOnly[j].TimeUpdated {
			return serverOnly[i].TimeUpdated > serverOnly[j].TimeUpdated
		}
		return serverOnly[i].Subscriptions > serverOnly[j].Subscriptions
	})
	return dedupeByFeature(localizeMods(serverOnly), limit), nil
}

func (a *app) searchWorkshopPaged(query string, sortName string, page int, pageSize int) ([]mod, bool, error) {
	localMatches := a.searchLocalMods(query)
	ids, err := a.scrapeWorkshopIDs(query, sortName, page)
	if err != nil {
		return nil, false, err
	}
	items, err := a.getWorkshopDetails(ids)
	if err != nil {
		return nil, false, err
	}
	serverOnly := filterServerOnly(localizeMods(items), len(items))
	sortMods(serverOnly, sortName)
	serverOnly = mergeMods(localMatches, serverOnly)
	start := (page - 1) * pageSize
	if start >= len(serverOnly) {
		return []mod{}, false, nil
	}
	end := start + pageSize
	if end > len(serverOnly) {
		end = len(serverOnly)
	}
	return serverOnly[start:end], end < len(serverOnly), nil
}

func (a *app) searchLocalMods(query string) []mod {
	query = normalizeSearchQuery(query)
	if query == "" {
		return []mod{}
	}
	candidates := a.loadLocalSearchMods()
	type scoredMod struct {
		item  mod
		score int
	}
	scored := make([]scoredMod, 0, len(candidates))
	for _, item := range candidates {
		if !item.Installable {
			continue
		}
		score := localModSearchScore(item, query)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredMod{item: item, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].item.TimeUpdated != scored[j].item.TimeUpdated {
			return scored[i].item.TimeUpdated > scored[j].item.TimeUpdated
		}
		return scored[i].item.Subscriptions > scored[j].item.Subscriptions
	})
	out := make([]mod, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.item)
	}
	return out
}

func (a *app) loadLocalSearchMods() []mod {
	var out []mod
	if s, err := a.loadStateFromMainFile(); err == nil {
		out = append(out, s.Mods...)
	}
	seedPath := filepath.Join(a.root, "mods", "server-mods.json")
	if s, err := loadModsFile(seedPath); err == nil {
		out = append(out, s.Mods...)
	}
	return mergeMods(out)
}

func loadModsFile(path string) (state, error) {
	var s state
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(data)) == 0 {
		return state{Mods: []mod{}}, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Mods == nil {
		s.Mods = []mod{}
	}
	for i := range s.Mods {
		s.Mods[i].ServerOnly = hasTag(s.Mods[i].Tags, "server_only_mod")
		s.Mods[i].ClientRequired = hasTag(s.Mods[i].Tags, "all_clients_require_mod")
		s.Mods[i].Installable = s.Mods[i].ServerOnly || s.Mods[i].ClientRequired
	}
	s.Mods = localizeMods(s.Mods)
	return s, nil
}

func localModSearchScore(item mod, query string) int {
	id := normalizeSearchQuery(item.ID)
	title := normalizeSearchQuery(item.Title)
	displayTitle := normalizeSearchQuery(item.DisplayTitle)
	category := normalizeSearchQuery(item.Category)
	description := normalizeSearchQuery(item.Description)
	tags := normalizeSearchQuery(strings.Join(item.Tags, " "))
	switch {
	case id == query:
		return 1100
	case displayTitle == query:
		return 1000
	case title == query:
		return 900
	case strings.Contains(displayTitle, query):
		return 800
	case strings.Contains(title, query):
		return 700
	case strings.Contains(category, query):
		return 500
	case strings.Contains(tags, query):
		return 300
	case strings.Contains(description, query):
		return 200
	default:
		return 0
	}
}

func normalizeSearchQuery(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mergeMods(groups ...[]mod) []mod {
	seen := map[string]bool{}
	out := []mod{}
	for _, group := range groups {
		for _, item := range group {
			id := strings.TrimSpace(item.ID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, item)
		}
	}
	return out
}

func sortMods(items []mod, sortName string) {
	sort.SliceStable(items, func(i, j int) bool {
		switch sortName {
		case "totaluniquesubscribers":
			if items[i].Subscriptions != items[j].Subscriptions {
				return items[i].Subscriptions > items[j].Subscriptions
			}
			return items[i].TimeUpdated > items[j].TimeUpdated
		default:
			if items[i].TimeUpdated != items[j].TimeUpdated {
				return items[i].TimeUpdated > items[j].TimeUpdated
			}
			return items[i].Subscriptions > items[j].Subscriptions
		}
	})
}

func normalizeWorkshopSort(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "subscriptions", "subscribers", "popular", "totaluniquesubscribers":
		return "totaluniquesubscribers"
	case "trend", "toprated":
		return strings.ToLower(strings.TrimSpace(input))
	default:
		return "timeupdated"
	}
}

func parseBoundedInt(input string, fallback int, minValue int, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (a *app) scrapeWorkshopIDs(query string, sortName string, pages int) ([]string, error) {
	seen := map[string]bool{}
	var ids []string
	for page := 1; page <= pages; page++ {
		values := url.Values{}
		values.Set("appid", appID)
		values.Set("browsesort", sortName)
		values.Set("section", "readytouseitems")
		values.Set("p", strconv.Itoa(page))
		values.Set("numperpage", "30")
		if query != "" {
			values.Set("searchtext", query)
		}
		browseURL := "https://steamcommunity.com/workshop/browse/?" + values.Encode() + "&requiredtags[]=server_only_mod"
		req, err := http.NewRequest(http.MethodGet, browseURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

		resp, err := a.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Steam Workshop returned %s", resp.Status)
		}
		for _, match := range idPattern.FindAllSubmatch(body, -1) {
			id := string(match[1])
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

func (a *app) getWorkshopDetails(ids []string) ([]mod, error) {
	filtered := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if numericPattern.MatchString(id) && !seen[id] {
			seen[id] = true
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return []mod{}, nil
	}

	values := url.Values{}
	values.Set("itemcount", strconv.Itoa(len(filtered)))
	for i, id := range filtered {
		values.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}

	resp, err := a.client.PostForm("https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Steam API returned %s", resp.Status)
	}

	var data detailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]mod, 0, len(data.Response.Details))
	for _, item := range data.Response.Details {
		id := strings.TrimSpace(item.PublishedFileID)
		if id == "" {
			continue
		}
		tags := make([]string, 0, len(item.Tags))
		for _, tag := range item.Tags {
			tags = append(tags, tag.Tag)
		}
		out = append(out, mod{
			ID:             id,
			Title:          item.Title,
			DisplayTitle:   displayTitle(id, item.Title),
			Category:       modCategory(id, item.Title),
			Description:    cleanDescription(item.Description),
			Preview:        item.PreviewURL,
			FileURL:        "https://steamcommunity.com/sharedfiles/filedetails/?id=" + id,
			Subscriptions:  item.Subscriptions,
			TimeUpdated:    item.TimeUpdated,
			Tags:           tags,
			ServerOnly:     hasTag(tags, "server_only_mod"),
			ClientRequired: hasTag(tags, "all_clients_require_mod"),
			Installable:    hasTag(tags, "server_only_mod") || hasTag(tags, "all_clients_require_mod"),
			Enabled:        true,
		})
	}
	return out, nil
}

func filterServerOnly(items []mod, limit int) []mod {
	out := make([]mod, 0, len(items))
	for _, item := range items {
		if hasTag(item.Tags, "server_only_mod") {
			out = append(out, item)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func localizeMods(items []mod) []mod {
	for i := range items {
		items[i].DisplayTitle = localizedDisplayTitle(items[i])
		items[i].Category = modCategory(items[i].ID, items[i].Title)
	}
	return items
}

func localizedDisplayTitle(item mod) string {
	if value, ok := knownDisplayTitles[item.ID]; ok {
		return value
	}
	if strings.TrimSpace(item.DisplayTitle) != "" {
		return item.DisplayTitle
	}
	return displayTitle(item.ID, item.Title)
}

func dedupeByFeature(items []mod, limit int) []mod {
	seen := map[string]bool{}
	out := make([]mod, 0, len(items))
	for _, item := range items {
		key := featureKey(item.ID, item.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func displayTitle(id string, title string) string {
	if value, ok := knownDisplayTitles[id]; ok {
		return value
	}
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "quick pick"):
		return "快速采集"
	case strings.Contains(lower, "quick action"):
		return "快速动作"
	case strings.Contains(lower, "icebox") || strings.Contains(title, "冰箱"):
		return "冰箱保鲜"
	case strings.Contains(lower, "don't drop"):
		return "死亡不掉落"
	case strings.Contains(lower, "thermal stone"):
		return "暖石无限耐久"
	case strings.Contains(lower, "tent"):
		return "帐篷无限使用"
	case strings.Contains(lower, "restart"):
		return "重选角色 / 重生指令"
	case strings.Contains(lower, "stack"):
		return "掉落自动堆叠"
	case strings.Contains(lower, "map"):
		return "地图全开"
	case strings.Contains(lower, "resurrection") || strings.Contains(lower, "respawn") || strings.Contains(title, "复活"):
		return "复活增强"
	case strings.Contains(lower, "door"):
		return "自动开门"
	case strings.Contains(lower, "health adjust"):
		return "生命值自动调整"
	case strings.Contains(lower, "trap"):
		return "陷阱自动重置"
	case strings.Contains(lower, "boss announcement") || strings.Contains(title, "公告"):
		return "Boss / 击杀公告"
	case strings.Contains(lower, "structure") || strings.Contains(title, "建筑无敌"):
		return "建筑无敌"
	case strings.Contains(lower, "regrowth") || strings.Contains(lower, "renewable"):
		return "世界资源再生"
	case strings.Contains(lower, "less lags"):
		return "服务器减卡顿"
	default:
		return title
	}
}

func modCategory(id string, title string) string {
	key := featureKey(id, title)
	switch key {
	case "drop-protection", "respawn", "restart":
		return "死亡与重生"
	case "quick-pick", "quick-actions", "fast-work", "auto-stack", "auto-door", "auto-trap":
		return "便捷操作"
	case "icebox", "thermal-stone", "tent":
		return "物品与耐久"
	case "map-reveal", "announcement", "tell-me":
		return "信息与公告"
	case "world-regrowth", "no-extinction", "no-grassgekko", "setpiece", "repair-antlion":
		return "世界设置"
	case "invincible-structures", "wall-regen", "fire-fight":
		return "建筑与防护"
	case "language":
		return "语言"
	case "server-admin", "less-lags":
		return "服务器管理"
	default:
		return "其他"
	}
}

func featureKey(id string, title string) string {
	if key, ok := knownFeatureKeys[id]; ok {
		return key
	}
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "quick pick"):
		return "quick-pick"
	case strings.Contains(lower, "quick action"):
		return "quick-actions"
	case strings.Contains(lower, "auto stack") || strings.Contains(lower, "drop & stack") || strings.Contains(title, "掉落堆叠"):
		return "auto-stack"
	case strings.Contains(lower, "icebox") || strings.Contains(title, "冰箱"):
		return "icebox"
	case strings.Contains(lower, "don't drop"):
		return "drop-protection"
	case strings.Contains(lower, "thermal stone"):
		return "thermal-stone"
	case strings.Contains(lower, "tent"):
		return "tent"
	case strings.Contains(lower, "restart"):
		return "restart"
	case strings.Contains(lower, "map reveal") || strings.Contains(title, "地图全开"):
		return "map-reveal"
	case strings.Contains(lower, "resurrection") || strings.Contains(lower, "respawn") || strings.Contains(title, "复活"):
		return "respawn"
	case strings.Contains(lower, "door"):
		return "auto-door"
	case strings.Contains(lower, "health adjust"):
		return "health-adjust"
	case strings.Contains(lower, "trap"):
		return "auto-trap"
	case strings.Contains(lower, "boss announcement") || strings.Contains(title, "公告"):
		return "announcement"
	case strings.Contains(lower, "invincible structure") || strings.Contains(title, "建筑无敌"):
		return "invincible-structures"
	case strings.Contains(lower, "regrowth") || strings.Contains(lower, "renewable"):
		return "world-regrowth"
	case strings.Contains(lower, "less lags"):
		return "less-lags"
	default:
		return "id:" + id
	}
}

var knownDisplayTitles = map[string]string{
	"661253977":  "死亡不掉落",
	"501385076":  "快速采集",
	"466732225":  "暖石无限耐久",
	"356930882":  "帐篷无限使用",
	"462434129":  "重选角色 / 重生指令",
	"1998081438": "掉落自动堆叠",
	"604761020":  "多倍岩石资源",
	"380423963":  "宝石可开采",
	"363112314":  "地图全开",
	"1898181913": "冰箱保鲜 / 反鲜",
	"1301033176": "服务器中文语言包",
	"676297854":  "火堆复活",
	"1803285852": "自动堆叠与拾取",
	"2074508776": "自动开门",
	"764204839":  "生命值自动调整",
	"648064643":  "加快工作速度",
	"631648169":  "击杀与 Boss 公告",
	"797304209":  "禁用草蜥蜴",
	"1845106626": "智能雪球发射器",
	"398858801":  "AFK 挂机检测",
	"514758022":  "世界资源再生",
	"1458450094": "陷阱自动重置",
	"2156905460": "修复蚁狮地陷",
	"597417408":  "服务器减卡顿",
	"3494615834": "三倍采集",
}

var knownFeatureKeys = map[string]string{
	"661253977":  "drop-protection",
	"2110246021": "drop-protection",
	"2845410726": "drop-protection",
	"501385076":  "quick-pick",
	"2921270365": "quick-pick",
	"1898181913": "icebox",
	"625415718":  "icebox",
	"541537428":  "icebox",
	"363112314":  "map-reveal",
	"2779367730": "map-reveal",
	"631648169":  "announcement",
	"1894295075": "announcement",
	"503795626":  "invincible-structures",
	"1614253006": "invincible-structures",
	"1458450094": "auto-trap",
	"679636739":  "auto-trap",
	"676297854":  "respawn",
	"2995466313": "respawn",
	"648064643":  "fast-work",
	"1751811434": "fast-work",
	"1301033176": "language",
	"2391292843": "language",
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

func (a *app) applyConfig() (map[string]any, error) {
	if a.dstDir == "" {
		return nil, fmt.Errorf("管理端未配置 -dst-dir，无法写入 DST 配置")
	}
	s, err := a.loadState()
	if err != nil {
		return nil, err
	}
	clusterDir, err := a.clusterDir()
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"Master", "Caves", "mods"} {
		if err := os.MkdirAll(filepath.Join(clusterDir, sub), 0o755); err != nil {
			return nil, err
		}
	}

	var enabled []mod
	for _, item := range s.Mods {
		if item.Enabled && numericPattern.MatchString(item.ID) {
			enabled = append(enabled, item)
		}
	}

	var setup strings.Builder
	setup.WriteString("-- Generated by cmd/mod-manager\n")
	if len(enabled) == 0 {
		setup.WriteString("-- No server mods enabled.\n")
	}
	for _, item := range enabled {
		setup.WriteString(fmt.Sprintf("ServerModSetup(%q)\n", item.ID))
	}

	var overrides strings.Builder
	overrides.WriteString("-- Generated by cmd/mod-manager\n")
	overrides.WriteString("return {\n")
	for _, item := range enabled {
		overrides.WriteString(fmt.Sprintf("  [%q] = { enabled = true }, -- %s\n", "workshop-"+item.ID, sanitizeLuaComment(item.Title)))
	}
	overrides.WriteString("}\n")

	written := []string{}
	writeFile := func(path string, data []byte) error {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		written = append(written, path)
		return nil
	}
	if err := writeFile(filepath.Join(clusterDir, "mods", "dedicated_server_mods_setup.lua"), []byte(setup.String())); err != nil {
		return nil, err
	}
	overridesData := []byte(overrides.String())
	if err := writeFile(filepath.Join(clusterDir, "Master", "modoverrides.lua"), overridesData); err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(clusterDir, "Caves", "modoverrides.lua"), overridesData); err != nil {
		return nil, err
	}

	settingsPaths, err := a.generateSettingsFiles(s.Settings)
	if err != nil {
		return nil, err
	}
	for _, p := range settingsPaths {
		written = append(written, p)
	}
	sort.Strings(written)
	return map[string]any{
		"status":        "ok",
		"enabled_count": len(enabled),
		"written":       written,
	}, nil
}

func (a *app) modDiagnostics() ([]modDiagnostic, error) {
	s, err := a.loadState()
	if err != nil {
		return nil, err
	}
	clusterDir, err := a.clusterDir()
	if err != nil {
		return nil, err
	}
	setupText := readTextIfExists(filepath.Join(clusterDir, "mods", "dedicated_server_mods_setup.lua"))
	masterOverrides := readTextIfExists(filepath.Join(clusterDir, "Master", "modoverrides.lua"))
	cavesOverrides := readTextIfExists(filepath.Join(clusterDir, "Caves", "modoverrides.lua"))
	logText := a.supervisorLogs() + "\n" +
		readTextIfExists(filepath.Join(clusterDir, "Master", "server_log.txt")) + "\n" +
		readTextIfExists(filepath.Join(clusterDir, "Caves", "server_log.txt"))

	out := make([]modDiagnostic, 0, len(s.Mods))
	for _, item := range s.Mods {
		if !numericPattern.MatchString(item.ID) {
			continue
		}
		diag := a.modDiagnostic(item, setupText, masterOverrides, cavesOverrides, logText)
		out = append(out, diag)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Enabled != out[j].Enabled {
			return out[i].Enabled
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (a *app) modDiagnostic(item mod, setupText string, masterOverrides string, cavesOverrides string, logText string) modDiagnostic {
	diag := modDiagnostic{
		ID:           item.ID,
		Title:        item.Title,
		DisplayTitle: displayTitle(item.ID, item.Title),
		Enabled:      item.Enabled,
		UpdatedAt:    formatUnixTime(item.TimeUpdated),
	}
	setupNeedle := fmt.Sprintf("ServerModSetup(%q)", item.ID)
	overrideNeedle := "workshop-" + item.ID
	diag.ConfiguredSetup = strings.Contains(setupText, setupNeedle)
	diag.ConfiguredOverride = strings.Contains(masterOverrides, overrideNeedle) || strings.Contains(cavesOverrides, overrideNeedle)
	diag.LoadedInLogs = strings.Contains(logText, "Loading mod: "+overrideNeedle) ||
		strings.Contains(logText, "ModIndex: Load "+overrideNeedle) ||
		strings.Contains(logText, overrideNeedle)

	if a.dstDir != "" {
		paths := []string{
			filepath.Join(a.dstDir, "ugc_mods", "content", appID, item.ID),
			filepath.Join(a.dstDir, "ugc_mods", "steamapps", "workshop", "content", appID, item.ID),
		}
		for _, path := range paths {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				diag.Paths = append(diag.Paths, path)
				diag.Downloaded = true
				if isLegacyWorkshopDir(path) {
					diag.LegacyPackage = true
				}
			}
		}
	}

	if item.Enabled && !diag.ConfiguredSetup {
		diag.Problems = append(diag.Problems, "没有写入 dedicated_server_mods_setup.lua，DST 不会自动下载")
	}
	if item.Enabled && !diag.ConfiguredOverride {
		diag.Problems = append(diag.Problems, "没有写入 Master/Caves 的 modoverrides.lua，DST 不会启用")
	}
	if item.Enabled && !diag.Downloaded {
		diag.Problems = append(diag.Problems, "本地没有找到已下载的 Workshop 文件")
	}
	if diag.LegacyPackage {
		diag.Problems = append(diag.Problems, "下载结果是 legacy 包，当前 DST 容器可能无法直接加载")
	}
	if item.Enabled && diag.Downloaded && diag.ConfiguredOverride && !diag.LoadedInLogs {
		diag.Problems = append(diag.Problems, "最近日志没有看到加载记录，可能需要重启或检查 MOD 错误")
	}
	if item.TimeUpdated > 0 {
		for _, path := range diag.Paths {
			if info, err := os.Stat(path); err == nil && info.ModTime().Unix()+86400 < item.TimeUpdated {
				diag.Outdated = true
				diag.Problems = append(diag.Problems, "本地下载时间早于 Workshop 更新时间，建议重新下载")
				break
			}
		}
	}
	return diag
}

func readTextIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > 4<<20 {
		data = data[len(data)-(4<<20):]
	}
	return string(data)
}

func isLegacyWorkshopDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) == 0 {
		return false
	}
	hasLegacy := false
	hasLua := false
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, "_legacy.bin") {
			hasLegacy = true
		}
		if name == "modinfo.lua" || name == "modmain.lua" {
			hasLua = true
		}
	}
	return hasLegacy && !hasLua
}

func formatUnixTime(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).Format("2006-01-02")
}

func (a *app) downloadWorkshopMod(id string) (modDownloadResponse, error) {
	if a.dstDir == "" {
		return modDownloadResponse{}, fmt.Errorf("管理端未配置 -dst-dir，无法下载 MOD")
	}
	steamcmd := os.Getenv("STEAMCMDDIR")
	if steamcmd == "" {
		steamcmd = "/home/steam/steamcmd"
	}
	steamcmdBin := filepath.Join(steamcmd, "steamcmd.sh")
	ugcRoot := filepath.Join(a.dstDir, "ugc_mods")
	if err := os.MkdirAll(ugcRoot, 0o755); err != nil {
		return modDownloadResponse{}, err
	}
	timeout := workshopDownloadTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Printf("Downloading Workshop mod %s via SteamCMD.", id)
	cmd := exec.CommandContext(
		ctx,
		steamcmdBin,
		"+@sSteamCmdForcePlatformType", "linux",
		"+force_install_dir", ugcRoot,
		"+login", "anonymous",
		"+workshop_download_item", appID, id,
		"+quit",
	)
	out, err := runCommandOutput(ctx, cmd)
	output := string(out)
	if ctx.Err() == context.DeadlineExceeded {
		return modDownloadResponse{}, fmt.Errorf("SteamCMD 下载超时（%s）：%s\n%s", timeout, id, tailText(output, 4000))
	}
	if err != nil {
		return modDownloadResponse{}, fmt.Errorf("SteamCMD 下载失败：%v\n%s", err, tailText(output, 4000))
	}
	log.Printf("SteamCMD finished Workshop mod %s download.", id)

	copyOutput, copyErr := a.copyDownloadedWorkshopMod(id)
	if copyOutput != "" {
		output = strings.TrimSpace(output + "\n" + copyOutput)
	}
	if copyErr != nil {
		return modDownloadResponse{}, copyErr
	}

	diag := a.diagnosticByID(id)
	status := "ok"
	message := "MOD 已通过 SteamCMD 下载"
	if diag.LegacyPackage {
		status = "legacy"
		message = "SteamCMD 下载成功，但这是 legacy 包，当前 DST 可能无法直接加载"
	} else if !diag.Downloaded {
		status = "warning"
		message = "SteamCMD 返回成功，但没有在目标目录找到 MOD 文件"
	}
	return modDownloadResponse{
		Status:     status,
		Message:    message,
		ID:         id,
		Output:     tailText(output, 4000),
		Diagnostic: diag,
	}, nil
}

func tailText(input string, maxBytes int) string {
	if maxBytes <= 0 || len(input) <= maxBytes {
		return input
	}
	return input[len(input)-maxBytes:]
}

func workshopDownloadTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("DST_WORKSHOP_DOWNLOAD_TIMEOUT"))
	if raw == "" {
		return 10 * time.Minute
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		log.Printf("Invalid DST_WORKSHOP_DOWNLOAD_TIMEOUT=%q; using 10m.", raw)
		return 10 * time.Minute
	}
	return value
}

func runCommandOutput(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done
		return out, ctx.Err()
	case <-done:
		return out, err
	}
}

func (a *app) copyDownloadedWorkshopMod(id string) (string, error) {
	ugcRoot := filepath.Join(a.dstDir, "ugc_mods")
	srcCandidates := []string{
		filepath.Join(ugcRoot, "steamapps", "workshop", "content", appID, id),
	}
	dst := filepath.Join(ugcRoot, "content", appID, id)
	for _, src := range srcCandidates {
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", err
			}
			if err := copyDir(src, dst); err != nil {
				return "", err
			}
			return fmt.Sprintf("copied %s to %s", src, dst), nil
		}
	}
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		return "downloaded directory already exists in " + dst, nil
	}
	return "", fmt.Errorf("SteamCMD 返回成功，但没有找到 Workshop 下载目录：%s", id)
}

func (a *app) diagnosticByID(id string) modDiagnostic {
	s, err := a.loadState()
	item := mod{ID: id, Title: "Workshop " + id, Enabled: true}
	if err == nil {
		for _, existing := range s.Mods {
			if existing.ID == id {
				item = existing
				break
			}
		}
	}
	clusterDir, err := a.clusterDir()
	if err != nil {
		return modDiagnostic{ID: id, Title: item.Title, DisplayTitle: displayTitle(id, item.Title), Problems: []string{err.Error()}}
	}
	return a.modDiagnostic(
		item,
		readTextIfExists(filepath.Join(clusterDir, "mods", "dedicated_server_mods_setup.lua")),
		readTextIfExists(filepath.Join(clusterDir, "Master", "modoverrides.lua")),
		readTextIfExists(filepath.Join(clusterDir, "Caves", "modoverrides.lua")),
		a.supervisorLogs(),
	)
}

func copyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src string, dst string, mode os.FileMode) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func (a *app) serverStatus() (serverStatusResponse, error) {
	checkedAt := time.Now().Format("2006-01-02 15:04:05")

	out, err := a.supervisorctl("status")
	logs := a.supervisorLogs()
	if err != nil && strings.TrimSpace(out) == "" {
		return serverStatusResponse{
			Status:    "error",
			Message:   strings.TrimSpace(fmt.Sprintf("读取 supervisor 状态失败：%v", err)),
			CheckedAt: checkedAt,
			Services:  []serviceStatus{},
			Logs:      logs,
		}, nil
	}

	services := parseSupervisorStatus(out)
	if !a.clusterTokenPresent() {
		return serverStatusResponse{
			Status:    "stopped",
			Message:   "未配置 Klei cluster token，DST 服务尚未启动",
			CheckedAt: checkedAt,
			Services:  services,
			Logs:      logs,
		}, nil
	}
	status, message := summarizeServices(services)
	return serverStatusResponse{
		Status:    status,
		Message:   message,
		CheckedAt: checkedAt,
		Services:  services,
		Logs:      logs,
	}, nil
}

func parseSupervisorStatus(data string) []serviceStatus {
	services := []serviceStatus{}
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		state := strings.ToLower(fields[1])
		rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, fields[1]))
		statusText := rest
		runningForText := ""
		if idx := strings.Index(rest, "uptime "); idx >= 0 {
			runningForText = strings.TrimSpace(rest[idx+len("uptime "):])
		}
		mappedState := state
		switch state {
		case "running":
			mappedState = "running"
		case "starting":
			mappedState = "starting"
		case "stopping":
			mappedState = "stopping"
		case "stopped", "exited":
			mappedState = "stopped"
		case "fatal", "backoff", "unknown":
			mappedState = "exited"
		}
		services = append(services, serviceStatus{
			Name:       name,
			Service:    name,
			State:      mappedState,
			Status:     statusText,
			RunningFor: runningForText,
		})
	}
	return services
}

func (a *app) supervisorLogs() string {
	out, err := a.supervisorctl("tail", "-1000", "dst-master")
	text := strings.TrimSpace(out)
	if err != nil {
		text = strings.TrimSpace(fmt.Sprintf("%v\n%s", err, text))
	}
	if len(text) > 20000 {
		text = text[len(text)-20000:]
	}
	return text
}

func summarizeServices(services []serviceStatus) (string, string) {
	if len(services) == 0 {
		return "unknown", "没有读取到 supervisor 进程，请检查容器启动情况"
	}

	running := 0
	starting := 0
	unhealthy := 0
	for _, service := range services {
		state := strings.ToLower(strings.TrimSpace(service.State))
		switch state {
		case "running":
			running++
		case "starting":
			starting++
		case "exited", "dead", "fatal":
			unhealthy++
		}
	}

	if unhealthy > 0 {
		return "error", "存在异常的 supervisor 进程，请查看日志"
	}
	if running == len(services) && starting == 0 {
		return "running", "服务已启动并正在运行"
	}
	if running > 0 || starting > 0 {
		return "starting", "进程正在启动，请稍后刷新"
	}
	return "stopped", "DST 进程未运行"
}

func (a *app) ensureDSTServerBinary() error {
	path := dstServerBinaryPath()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("DST 服务端文件尚未安装：缺少 %s。请先让镜像/容器完成 SteamCMD 下载 343050，然后再启动服务器", path)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("DST 服务端文件不可执行：%s", path)
	}
	return nil
}

func dstServerBinaryPath() string {
	gameDir := strings.TrimSpace(os.Getenv("DST_GAME_DIR"))
	if gameDir == "" {
		gameDir = "/opt/dst/game"
	}
	return filepath.Join(gameDir, "bin64", "dontstarve_dedicated_server_nullrenderer_x64")
}

func (a *app) currentSupervisorServices() ([]serviceStatus, error) {
	out, err := a.supervisorctl("status")
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, err
	}
	return parseSupervisorStatus(out), nil
}

func supervisorStartAction(name string, services []serviceStatus) string {
	for _, service := range services {
		if service.Name == name && strings.EqualFold(service.State, "running") {
			return "restart"
		}
	}
	return "start"
}

func (a *app) clusterDir() (string, error) {
	if a.dstDir == "" {
		return "", fmt.Errorf("管理端未配置 -dst-dir，无法读取 DST 配置")
	}
	return filepath.Join(a.dstDir, "cluster", "Cluster_1"), nil
}

func (a *app) adminListPath() (string, error) {
	clusterDir, err := a.clusterDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(clusterDir, "adminlist.txt"), nil
}

func (a *app) readAdminSet() (map[string]bool, error) {
	path, err := a.adminListPath()
	if err != nil {
		return nil, err
	}
	admins := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return admins, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		id := strings.TrimSpace(line)
		if kuidExactPattern.MatchString(id) {
			admins[id] = true
		}
	}
	return admins, nil
}

func (a *app) writeAdminSet(admins map[string]bool) error {
	path, err := a.adminListPath()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(admins))
	for id := range admins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(ids, "\n")+"\n"), 0o644)
}

func (a *app) loadPlayersFromLogs() ([]playerInfo, error) {
	clusterDir, err := a.clusterDir()
	if err != nil {
		return nil, err
	}
	admins, err := a.readAdminSet()
	if err != nil {
		return nil, err
	}
	logPaths := []string{
		filepath.Join(clusterDir, "Master", "server_log.txt"),
		filepath.Join(clusterDir, "Master", "server_chat_log.txt"),
		filepath.Join(clusterDir, "Caves", "server_log.txt"),
		filepath.Join(clusterDir, "Caves", "server_chat_log.txt"),
	}
	players := map[string]playerInfo{}
	for _, path := range logPaths {
		data, modTime, err := readTail(path, 2<<20)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		lastSeen := modTime.Format("2006-01-02 15:04:05")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "Directory [") || strings.Contains(line, "DelDirectory [") {
				continue
			}
			ids := kuidPattern.FindAllString(line, -1)
			for _, id := range ids {
				name := extractPlayerName(line, id)
				if existing, ok := players[id]; ok && existing.Name != "" && existing.Name != "未知玩家" {
					name = existing.Name
				}
				players[id] = playerInfo{
					KUID:     id,
					Name:     name,
					LastSeen: lastSeen,
					Admin:    admins[id],
				}
			}
		}
	}
	for id := range admins {
		if _, ok := players[id]; !ok {
			players[id] = playerInfo{
				KUID:     id,
				Name:     "未知玩家",
				LastSeen: "管理员列表",
				Admin:    true,
			}
		}
	}
	out := make([]playerInfo, 0, len(players))
	for _, player := range players {
		out = append(out, player)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Admin != out[j].Admin {
			return out[i].Admin
		}
		return out[i].KUID < out[j].KUID
	})
	return out, nil
}

func readTail(path string, maxBytes int64) ([]byte, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer file.Close()
	size := info.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, time.Time{}, err
	}
	data, err := io.ReadAll(file)
	return data, info.ModTime(), err
}

func extractPlayerName(line string, kuid string) string {
	authNeedle := "Client authenticated: (" + kuid + ")"
	if idx := strings.Index(line, authNeedle); idx >= 0 {
		name := cleanPlayerName(line[idx+len(authNeedle):])
		if name != "" {
			return name
		}
	}
	if match := namedFieldPattern.FindStringSubmatch(line); len(match) > 1 {
		return cleanPlayerName(match[1])
	}
	quoted := quotedNamePattern.FindAllStringSubmatch(line, -1)
	for _, match := range quoted {
		name := cleanPlayerName(match[1])
		if name != "" && !strings.Contains(name, "KU_") {
			return name
		}
	}
	return "未知玩家"
}

func cleanPlayerName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "'\"[]()")
	name = whitespacePattern.ReplaceAllString(name, " ")
	if name == "" || strings.Contains(name, "/") {
		return ""
	}
	return name
}

func (a *app) generateSettingsFiles(settings serverSettings) (map[string]string, error) {
	settings = normalizeSettings(settings)
	clusterDir, err := a.clusterDir()
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"Master", "Caves"} {
		if err := os.MkdirAll(filepath.Join(clusterDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(a.settingsFile), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(a.settingsFile, append(settingsJSON, '\n'), 0o644); err != nil {
		return nil, err
	}

	clusterPath := filepath.Join(clusterDir, "cluster.ini")
	clusterINI := fmt.Sprintf(`[GAMEPLAY]
game_mode = %s
max_players = %d
pvp = %s
pause_when_empty = %s

[NETWORK]
cluster_description = Dedicated Don't Starve Together server.
cluster_name = %s
cluster_password = %s
cluster_intention = cooperative
offline_cluster = false
lan_only_cluster = false

[MISC]
console_enabled = true
max_snapshots = 6

[SHARD]
shard_enabled = %s
bind_ip = 127.0.0.1
master_ip = 127.0.0.1
master_port = 10998
cluster_key = dst_cluster_key
`, settings.GameMode, settings.MaxPlayers, boolString(settings.PVP), boolString(settings.PauseWhenEmpty), settings.ServerName, settings.ServerPassword, boolString(settings.EnableCaves))
	if err := os.WriteFile(clusterPath, []byte(clusterINI), 0o644); err != nil {
		return nil, err
	}

	masterPath := filepath.Join(clusterDir, "Master", "server.ini")
	masterINI := `[SHARD]
is_master = true

[STEAM]
authentication_port = 8768
master_server_port = 27018

[NETWORK]
server_port = 10999
`
	if err := os.WriteFile(masterPath, []byte(masterINI), 0o644); err != nil {
		return nil, err
	}

	cavesPath := filepath.Join(clusterDir, "Caves", "server.ini")
	cavesINI := `[SHARD]
is_master = false
name = Caves

[STEAM]
authentication_port = 8769
master_server_port = 27019

[NETWORK]
server_port = 11000
`
	if err := os.WriteFile(cavesPath, []byte(cavesINI), 0o644); err != nil {
		return nil, err
	}

	return map[string]string{
		"server_settings":   a.settingsFile,
		"cluster_ini":       clusterPath,
		"master_server_ini": masterPath,
		"caves_server_ini":  cavesPath,
	}, nil
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *app) supervisorctl(args ...string) (string, error) {
	cmdArgs := []string{}
	if a.supervisorConf != "" {
		cmdArgs = append(cmdArgs, "-c", a.supervisorConf)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("supervisorctl", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (a *app) clusterTokenPresent() bool {
	clusterDir, err := a.clusterDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(clusterDir, "cluster_token.txt"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) != ""
}

func cleanDescription(input string) string {
	noHTML := htmlTagPattern.ReplaceAllString(input, " ")
	clean := whitespacePattern.ReplaceAllString(noHTML, " ")
	clean = strings.TrimSpace(clean)
	if len(clean) > 700 {
		clean = clean[:700] + "..."
	}
	return clean
}

func sanitizeLuaComment(input string) string {
	return strings.ReplaceAll(input, "--", "")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
