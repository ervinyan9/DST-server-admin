package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type tokenStatus struct {
	Present       bool   `json:"present"`
	FilePresent   bool   `json:"file_present"`
	Fingerprint   string `json:"fingerprint"`
	SavedAt       string `json:"saved_at"`
	FileWrittenAt string `json:"file_written_at"`
	LastStatus    string `json:"last_status"`
	LastMessage   string `json:"last_message"`
	LastCheckedAt string `json:"last_checked_at"`
}

type clusterTokenRecord struct {
	Token         string
	Fingerprint   string
	SavedAt       string
	FileWrittenAt string
	LastStatus    string
	LastMessage   string
	LastCheckedAt string
}

func (a *app) ensureStore() error {
	if a.dbPath == "" {
		return nil
	}
	if a.db != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.dbPath), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(a.dbPath), 0o700); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", a.dbPath)
	if err != nil {
		return err
	}
	a.db = db
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return err
	}
	if err := a.migrateStore(); err != nil {
		return err
	}
	return os.Chmod(a.dbPath, 0o600)
}

func (a *app) migrateStore() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS cluster_token (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			token TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			saved_at TEXT NOT NULL,
			file_written_at TEXT NOT NULL DEFAULT '',
			last_status TEXT NOT NULL DEFAULT 'saved',
			last_message TEXT NOT NULL DEFAULT '已保存，等待重启验证',
			last_checked_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS runtime_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := a.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) importLegacyStateIfEmpty() error {
	if err := a.ensureStore(); err != nil {
		return err
	}
	hasState, err := a.storeHasState()
	if err != nil {
		return err
	}
	if !hasState {
		s := state{Mods: []mod{}, Settings: defaultSettings()}
		if loaded, loadErr := a.loadStateFromMainFile(); loadErr == nil {
			s = loaded
		} else if !os.IsNotExist(loadErr) {
			return loadErr
		}
		settings, settingsErr := a.loadSettings(s.Settings)
		if settingsErr != nil {
			return settingsErr
		}
		s.Settings = settings
		if err := a.saveStateToStore(s); err != nil {
			return err
		}
		if err := a.insertRuntimeEvent("state_import", "已初始化 SQLite 管理端状态"); err != nil {
			return err
		}
	}
	if _, err := a.loadClusterTokenRecord(); errors.Is(err, sql.ErrNoRows) {
		if token := strings.TrimSpace(a.readClusterTokenFile()); token != "" {
			if err := a.saveClusterTokenToStore(token); err != nil {
				return err
			}
			if err := a.markClusterTokenFileWritten(); err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (a *app) storeHasState() (bool, error) {
	if err := a.ensureStore(); err != nil {
		return false, err
	}
	var count int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM app_state WHERE key IN ('mods', 'settings')`).Scan(&count)
	return count > 0, err
}

func (a *app) loadStateFromStore() (state, error) {
	if err := a.ensureStore(); err != nil {
		return state{}, err
	}
	s := state{Mods: []mod{}, Settings: defaultSettings()}
	if raw, err := a.storeValue("mods"); err == nil && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &s.Mods); err != nil {
			return state{}, err
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return state{}, err
	}
	if raw, err := a.storeValue("settings"); err == nil && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &s.Settings); err != nil {
			return state{}, err
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return state{}, err
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

func (a *app) saveStateToStore(s state) error {
	if err := a.ensureStore(); err != nil {
		return err
	}
	if s.Mods == nil {
		s.Mods = []mod{}
	}
	s.Settings = normalizeSettings(s.Settings)
	modsData, err := json.Marshal(s.Mods)
	if err != nil {
		return err
	}
	settingsData, err := json.Marshal(s.Settings)
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Format(time.RFC3339Nano)
	if _, err := tx.Exec(upsertAppStateSQL(), "mods", string(modsData), now); err != nil {
		return err
	}
	if _, err := tx.Exec(upsertAppStateSQL(), "settings", string(settingsData), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *app) saveSettingsToStore(settings serverSettings) error {
	s, err := a.loadStateFromStore()
	if err != nil {
		return err
	}
	s.Settings = normalizeSettings(settings)
	return a.saveStateToStore(s)
}

func (a *app) storeValue(key string) (string, error) {
	if err := a.ensureStore(); err != nil {
		return "", err
	}
	var value string
	err := a.db.QueryRow(`SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	return value, err
}

func upsertAppStateSQL() string {
	return `INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at`
}

func (a *app) saveClusterTokenToStore(token string) error {
	if err := a.ensureStore(); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	fingerprint := tokenFingerprint(token)
	_, err := a.db.Exec(`INSERT INTO cluster_token
		(id, token, fingerprint, saved_at, file_written_at, last_status, last_message, last_checked_at)
		VALUES (1, ?, ?, ?, '', 'saved', '已保存，等待重启验证', ?)
		ON CONFLICT(id) DO UPDATE SET
			token = excluded.token,
			fingerprint = excluded.fingerprint,
			saved_at = excluded.saved_at,
			file_written_at = '',
			last_status = 'saved',
			last_message = '已保存，等待重启验证',
			last_checked_at = excluded.last_checked_at`,
		token, fingerprint, now, now)
	if err != nil {
		return err
	}
	return a.insertRuntimeEvent("cluster_token_saved", "Klei cluster token 已保存到 SQLite")
}

func (a *app) markClusterTokenFileWritten() error {
	if err := a.ensureStore(); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err := a.db.Exec(`UPDATE cluster_token SET file_written_at = ? WHERE id = 1`, now)
	return err
}

func (a *app) loadClusterTokenPlaintext() (string, error) {
	record, err := a.loadClusterTokenRecord()
	if err != nil {
		return "", err
	}
	return record.Token, nil
}

func (a *app) loadClusterTokenRecord() (clusterTokenRecord, error) {
	if err := a.ensureStore(); err != nil {
		return clusterTokenRecord{}, err
	}
	var record clusterTokenRecord
	err := a.db.QueryRow(`SELECT token, fingerprint, saved_at, file_written_at, last_status, last_message, last_checked_at
		FROM cluster_token WHERE id = 1`).Scan(
		&record.Token,
		&record.Fingerprint,
		&record.SavedAt,
		&record.FileWrittenAt,
		&record.LastStatus,
		&record.LastMessage,
		&record.LastCheckedAt,
	)
	return record, err
}

func (a *app) currentTokenStatus() tokenStatus {
	filePresent := strings.TrimSpace(a.readClusterTokenFile()) != ""
	if a.dbPath == "" {
		return tokenStatus{Present: filePresent, FilePresent: filePresent}
	}
	record, err := a.loadClusterTokenRecord()
	if err != nil {
		return tokenStatus{Present: false, FilePresent: filePresent, LastStatus: "missing", LastMessage: "未保存 Klei cluster token"}
	}
	return tokenStatus{
		Present:       strings.TrimSpace(record.Token) != "",
		FilePresent:   filePresent,
		Fingerprint:   record.Fingerprint,
		SavedAt:       record.SavedAt,
		FileWrittenAt: record.FileWrittenAt,
		LastStatus:    record.LastStatus,
		LastMessage:   record.LastMessage,
		LastCheckedAt: record.LastCheckedAt,
	}
}

func (a *app) recordTokenStatus(status string, message string) tokenStatus {
	if a.dbPath == "" {
		return tokenStatus{
			Present:       a.clusterTokenPresent(),
			FilePresent:   strings.TrimSpace(a.readClusterTokenFile()) != "",
			LastStatus:    status,
			LastMessage:   message,
			LastCheckedAt: time.Now().Format(time.RFC3339Nano),
		}
	}
	if err := a.ensureStore(); err != nil {
		return a.currentTokenStatus()
	}
	now := time.Now().Format(time.RFC3339Nano)
	_, err := a.db.Exec(`UPDATE cluster_token
		SET last_status = ?, last_message = ?, last_checked_at = ?
		WHERE id = 1`, status, message, now)
	if err == nil {
		_ = a.insertRuntimeEvent("cluster_token_status", fmt.Sprintf("%s: %s", status, message))
	}
	return a.currentTokenStatus()
}

func (a *app) tokenLogProblemIsCurrent(status tokenStatus) bool {
	return a.tokenLogIsCurrent(status)
}

func (a *app) tokenLogIsCurrent(status tokenStatus) bool {
	if strings.TrimSpace(status.SavedAt) == "" {
		return true
	}
	savedAt, err := time.Parse(time.RFC3339Nano, status.SavedAt)
	if err != nil {
		return true
	}
	logTime := a.latestDSTLogModTime()
	if logTime.IsZero() {
		return false
	}
	return !savedAt.After(logTime)
}

func (a *app) latestDSTLogModTime() time.Time {
	clusterDir, err := a.clusterDir()
	if err != nil {
		return time.Time{}
	}
	var latest time.Time
	for _, shard := range []string{"Master", "Caves"} {
		info, err := os.Stat(filepath.Join(clusterDir, shard, "server_log.txt"))
		if err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func (a *app) restoreClusterTokenFile() error {
	if a.dbPath == "" {
		return nil
	}
	token, err := a.loadClusterTokenPlaintext()
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if strings.TrimSpace(a.readClusterTokenFile()) == token {
		return nil
	}
	clusterDir, err := a.clusterDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster_token.txt"), []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	if err := a.markClusterTokenFileWritten(); err != nil {
		return err
	}
	return a.insertRuntimeEvent("cluster_token_restored", "已从 SQLite 恢复 cluster_token.txt")
}

func (a *app) readClusterTokenFile() string {
	clusterDir, err := a.clusterDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(clusterDir, "cluster_token.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (a *app) insertRuntimeEvent(eventType string, message string) error {
	if a.dbPath == "" {
		return nil
	}
	if err := a.ensureStore(); err != nil {
		return err
	}
	_, err := a.db.Exec(`INSERT INTO runtime_events (event_type, message, created_at) VALUES (?, ?, ?)`,
		eventType, message, time.Now().Format(time.RFC3339Nano))
	return err
}

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	out := hex.EncodeToString(sum[:])
	if len(out) > 12 {
		return out[:12]
	}
	return out
}
