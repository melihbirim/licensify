package proxy

import (
"context"
"database/sql"
"encoding/json"
"fmt"
"io"
"log"
"net/http"
"strings"
"time"
)

type Provider struct {
Name       string
BaseURL    string
APIKey     string
PathPrefix string
Headers    func(string) map[string]string
}

type Handler struct {
keyManager  *KeyManager
db          *sql.DB
isPostgres  bool
placeholder func(int) string
providers   map[string]*Provider
}

type License struct {
ID           string
Tier         string
DailyLimit   int
MonthlyLimit int
ExpiresAt    time.Time
}

type Usage struct {
Daily   int
Monthly int
}

func NewHandler(km *KeyManager, db *sql.DB, isPostgres bool, placeholder func(int) string) *Handler {
return &Handler{keyManager: km, db: db, isPostgres: isPostgres, placeholder: placeholder, providers: make(map[string]*Provider)}
}

func (h *Handler) RegisterProvider(name, baseURL, apiKey, pathPrefix string, headers func(string) map[string]string) {
h.providers[name] = &Provider{Name: name, BaseURL: baseURL, APIKey: apiKey, PathPrefix: pathPrefix, Headers: headers}
}

func (h *Handler) RegisterOpenAI(apiKey string) {
h.RegisterProvider("openai", "https://api.openai.com", apiKey, "/proxy/openai", func(key string) map[string]string {
return map[string]string{"Authorization": "Bearer " + key, "Content-Type": "application/json"}
})
}

func (h *Handler) RegisterAnthropic(apiKey string) {
h.RegisterProvider("anthropic", "https://api.anthropic.com", apiKey, "/proxy/anthropic", func(key string) map[string]string {
return map[string]string{"x-api-key": key, "anthropic-version": "2023-06-01", "Content-Type": "application/json"}
})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

var req Request
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
sendError(w, "Invalid request body", http.StatusBadRequest)
return
}

if !strings.HasPrefix(req.ProxyKey, "px_") {
sendError(w, "Invalid proxy key format", http.StatusBadRequest)
return
}

if !ValidateSignature(req.ProxyKey, req.Provider, req.Body, req.Timestamp, req.Signature) {
log.Printf("Invalid proxy signature for key: %s", redactKey(req.ProxyKey))
sendError(w, "Invalid signature or expired timestamp", http.StatusUnauthorized)
return
}

licenseKey, hardwareID, err := h.keyManager.Validate(req.ProxyKey)
if err != nil {
if err == sql.ErrNoRows {
log.Printf("Proxy key not found: %s", redactKey(req.ProxyKey))
sendError(w, "Unauthorized", http.StatusUnauthorized)
} else {
log.Printf("Database error validating proxy key: %v", err)
sendError(w, "Internal server error", http.StatusInternalServerError)
}
return
}

license, err := h.getLicense(licenseKey)
if err != nil {
if err == sql.ErrNoRows {
sendError(w, "License not found or inactive", http.StatusUnauthorized)
} else {
log.Printf("Database error: %v", err)
sendError(w, "Internal server error", http.StatusInternalServerError)
}
return
}

if time.Now().After(license.ExpiresAt) {
sendError(w, "License has expired", http.StatusUnauthorized)
return
}

if err := h.checkActivation(licenseKey, hardwareID); err != nil {
sendError(w, "Hardware ID not activated for this license", http.StatusUnauthorized)
return
}

usage, err := h.checkRateLimit(licenseKey, hardwareID, license.DailyLimit, license.MonthlyLimit)
if err != nil {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusTooManyRequests)
json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]interface{}{"message": err.Error(), "type": "rate_limit_exceeded"}})
return
}

provider, ok := h.providers[req.Provider]
if !ok {
sendError(w, "Unsupported provider", http.StatusBadRequest)
return
}

if len(req.Body) > 1024*1024 {
sendError(w, "Request body too large", http.StatusRequestEntityTooLarge)
return
}

path := strings.TrimPrefix(r.URL.Path, provider.PathPrefix)
if path == "" || path == "/" {
path = getDefaultPath(req.Provider)
}
apiURL := provider.BaseURL + path

ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
defer cancel()

proxyReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(req.Body)))
if err != nil {
log.Printf("Failed to create proxy request: %v", err)
sendError(w, "Internal server error", http.StatusInternalServerError)
return
}

for key, value := range provider.Headers(provider.APIKey) {
proxyReq.Header.Set(key, value)
}

client := &http.Client{Timeout: 60 * time.Second}
resp, err := client.Do(proxyReq)
if err != nil {
if ctx.Err() == context.DeadlineExceeded {
sendError(w, "Request timeout", http.StatusGatewayTimeout)
} else {
sendError(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
}
return
}
defer resp.Body.Close()

h.incrementUsage(licenseKey, hardwareID)

for key, values := range resp.Header {
for _, value := range values {
w.Header().Add(key, value)
}
}

w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", license.DailyLimit))
w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", license.DailyLimit-usage.Daily-1))
w.Header().Set("X-RateLimit-Reset", time.Now().Add(24*time.Hour).Format(time.RFC3339))

w.WriteHeader(resp.StatusCode)
io.Copy(w, resp.Body)
}

func (h *Handler) getLicense(licenseID string) (*License, error) {
var license License
license.ID = licenseID

if h.isPostgres {
var expiresAtUnix int64
err := h.db.QueryRow(fmt.Sprintf(`SELECT tier, daily_limit, monthly_limit, EXTRACT(EPOCH FROM expires_at)::bigint FROM licenses WHERE license_id = %s AND active = true`, h.placeholder(1)), licenseID).Scan(&license.Tier, &license.DailyLimit, &license.MonthlyLimit, &expiresAtUnix)
if err != nil {
return nil, err
}
license.ExpiresAt = time.Unix(expiresAtUnix, 0)
} else {
var expiresAtStr string
err := h.db.QueryRow(fmt.Sprintf(`SELECT tier, daily_limit, monthly_limit, expires_at FROM licenses WHERE license_id = %s AND active = true`, h.placeholder(1)), licenseID).Scan(&license.Tier, &license.DailyLimit, &license.MonthlyLimit, &expiresAtStr)
if err != nil {
return nil, err
}
license.ExpiresAt, err = time.Parse(time.RFC3339, expiresAtStr)
if err != nil {
return nil, err
}
}

return &license, nil
}

func (h *Handler) checkActivation(licenseID, hardwareID string) error {
var count int
err := h.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM activations WHERE license_id = %s AND hardware_id = %s`, h.placeholder(1), h.placeholder(2)), licenseID, hardwareID).Scan(&count)
if err != nil {
return err
}
if count == 0 {
return fmt.Errorf("hardware not activated")
}
return nil
}

func (h *Handler) checkRateLimit(licenseID, hardwareID string, dailyLimit, monthlyLimit int) (*Usage, error) {
today := time.Now().Format("2006-01-02")
var dailyUsage int
err := h.db.QueryRow(fmt.Sprintf(`SELECT scans FROM daily_usage WHERE license_id = %s AND date = %s AND hardware_id = %s`, h.placeholder(1), h.placeholder(2), h.placeholder(3)), licenseID, today, hardwareID).Scan(&dailyUsage)
if err != nil && err != sql.ErrNoRows {
return nil, err
}

if dailyUsage >= dailyLimit {
return nil, fmt.Errorf("Daily limit of %d requests exceeded. Current usage: %d", dailyLimit, dailyUsage)
}

var monthlyUsage int
if monthlyLimit > 0 {
thisMonth := time.Now().Format("2006-01")
err = h.db.QueryRow(fmt.Sprintf(`SELECT COALESCE(SUM(scans), 0) FROM daily_usage WHERE license_id = %s AND hardware_id = %s AND date LIKE %s`, h.placeholder(1), h.placeholder(2), h.placeholder(3)), licenseID, hardwareID, thisMonth+"%").Scan(&monthlyUsage)
if err != nil {
return nil, err
}
if monthlyUsage >= monthlyLimit {
return nil, fmt.Errorf("Monthly limit of %d requests exceeded. Current usage: %d", monthlyLimit, monthlyUsage)
}
}

return &Usage{Daily: dailyUsage, Monthly: monthlyUsage}, nil
}

func (h *Handler) incrementUsage(licenseID, hardwareID string) {
today := time.Now().Format("2006-01-02")
_, err := h.db.Exec(fmt.Sprintf(`INSERT INTO daily_usage (license_id, date, scans, hardware_id) VALUES (%s, %s, 1, %s) ON CONFLICT (license_id, date) DO UPDATE SET scans = daily_usage.scans + 1`, h.placeholder(1), h.placeholder(2), h.placeholder(3)), licenseID, today, hardwareID)
if err != nil {
log.Printf("Failed to update usage: %v", err)
}
}

func getDefaultPath(provider string) string {
switch provider {
case "openai":
return "/v1/chat/completions"
case "anthropic":
return "/v1/messages"
default:
return "/"
}
}

func sendError(w http.ResponseWriter, message string, statusCode int) {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(statusCode)
json.NewEncoder(w).Encode(map[string]string{"error": message})
}
