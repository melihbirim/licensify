package proxy

import (
"database/sql"
"encoding/json"
"net/http"
"net/http/httptest"
"strings"
"testing"
"time"
_ "github.com/mattn/go-sqlite3"
)

func setupDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	schema := `
	CREATE TABLE proxy_keys (proxy_key TEXT PRIMARY KEY, license_id TEXT, hardware_id TEXT);
	CREATE TABLE licenses (license_id TEXT PRIMARY KEY, tier TEXT, expires_at TEXT, daily_limit INTEGER, monthly_limit INTEGER, active BOOLEAN);
	CREATE TABLE activations (license_id TEXT, hardware_id TEXT);
	CREATE TABLE daily_usage (license_id TEXT, date TEXT, scans INTEGER, hardware_id TEXT, PRIMARY KEY (license_id, date));
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db, func() { db.Close() }
}

func placeholder(n int) string {
	return "?"
}

func TestKeyManagerGenerate(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	key1, err := km.Generate()
	if err != nil || !strings.HasPrefix(key1, "px_") || len(key1) < 40 {
		t.Errorf("Generate failed: %v, key=%s", err, key1)
	}
	key2, _ := km.Generate()
	if key1 == key2 {
		t.Error("Expected unique keys")
	}
}

func TestKeyManagerStore(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	key, _ := km.Generate()
	if err := km.Store(key, "LIC-123", "hw-001"); err != nil {
		t.Fatal(err)
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM proxy_keys WHERE proxy_key = ?", key).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 key, got %d", count)
	}
}

func TestKeyManagerValidate(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	key, _ := km.Generate()
	km.Store(key, "LIC-456", "hw-002")
	licenseID, hardwareID, err := km.Validate(key)
	if err != nil || licenseID != "LIC-456" || hardwareID != "hw-002" {
		t.Errorf("Validate failed: %v, got %s/%s", err, licenseID, hardwareID)
	}
}

func TestHandlerRegisterOpenAI(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	handler.RegisterOpenAI("sk-test")
	if provider, ok := handler.providers["openai"]; !ok || provider.BaseURL != "https://api.openai.com" {
		t.Error("OpenAI registration failed")
	}
}

func TestHandlerRegisterAnthropic(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	handler.RegisterAnthropic("sk-ant-test")
	if provider, ok := handler.providers["anthropic"]; !ok {
		t.Error("Anthropic registration failed")
	} else {
		headers := provider.Headers("test-key")
		if headers["x-api-key"] != "test-key" || headers["anthropic-version"] != "2023-06-01" {
			t.Error("Anthropic headers incorrect")
		}
	}
}

func TestGetDefaultPath(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{"openai", "/v1/chat/completions"},
		{"anthropic", "/v1/messages"},
		{"unknown", "/"},
	}
	for _, tt := range tests {
		if result := getDefaultPath(tt.provider); result != tt.expected {
			t.Errorf("getDefaultPath(%s) = %s, want %s", tt.provider, result, tt.expected)
		}
	}
}

func TestRedactKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"px_abcdefghijklmnop", "px_abcd..."},
		{"short", "***"},
	}
	for _, tt := range tests {
		if result := redactKey(tt.input); result != tt.expected {
			t.Errorf("redactKey(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestHandlerServeHTTPMethodNotAllowed(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	req := httptest.NewRequest("GET", "/proxy/openai", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestHandlerServeHTTPInvalidKeyFormat(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	body := `{"proxy_key":"invalid","provider":"openai","body":{},"signature":"test","timestamp":0}`
	req := httptest.NewRequest("POST", "/proxy/openai", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestHandlerCheckActivation(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	db.Exec("INSERT INTO activations (license_id, hardware_id) VALUES (?, ?)", "LIC-123", "hw-001")
	if err := handler.checkActivation("LIC-123", "hw-001"); err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if err := handler.checkActivation("LIC-123", "hw-999"); err == nil {
		t.Error("Expected error for non-activated hardware")
	}
}

func TestHandlerGetLicense(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	expiresAt := time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339)
	db.Exec("INSERT INTO licenses (license_id, tier, expires_at, daily_limit, monthly_limit, active) VALUES (?, ?, ?, ?, ?, ?)", "LIC-123", "pro", expiresAt, 100, 3000, true)
	license, err := handler.getLicense("LIC-123")
	if err != nil || license.ID != "LIC-123" || license.DailyLimit != 100 {
		t.Errorf("Failed to get license: %v", err)
	}
}

func TestHandlerCheckRateLimit(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	today := time.Now().Format("2006-01-02")
	db.Exec("INSERT INTO daily_usage (license_id, date, scans, hardware_id) VALUES (?, ?, ?, ?)", "LIC-123", today, 5, "hw-001")
	usage, err := handler.checkRateLimit("LIC-123", "hw-001", 100, 3000)
	if err != nil || usage.Daily != 5 {
		t.Errorf("Rate limit check failed: %v", err)
	}
	if _, err := handler.checkRateLimit("LIC-123", "hw-001", 5, 3000); err == nil {
		t.Error("Expected rate limit error")
	}
}

func TestHandlerIncrementUsage(t *testing.T) {
	db, cleanup := setupDB(t)
	defer cleanup()
	km := NewKeyManager(db, placeholder)
	handler := NewHandler(km, db, false, placeholder)
	handler.incrementUsage("LIC-123", "hw-001")
	today := time.Now().Format("2006-01-02")
	var scans int
	db.QueryRow("SELECT scans FROM daily_usage WHERE license_id = ? AND date = ?", "LIC-123", today).Scan(&scans)
	if scans != 1 {
		t.Errorf("Expected 1 scan, got %d", scans)
	}
	handler.incrementUsage("LIC-123", "hw-001")
	db.QueryRow("SELECT scans FROM daily_usage WHERE license_id = ? AND date = ?", "LIC-123", today).Scan(&scans)
	if scans != 2 {
		t.Errorf("Expected 2 scans, got %d", scans)
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	sendError(w, "test error", http.StatusBadRequest)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)
	if response["error"] != "test error" {
		t.Errorf("Expected 'test error', got %v", response["error"])
	}
}
