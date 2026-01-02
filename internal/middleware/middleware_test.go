package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestNewRateLimiter(t *testing.T) {
	config := DefaultRateLimiterConfig()
	rl := NewRateLimiter(config)
	defer rl.Stop()

	if rl == nil {
		t.Fatal("Expected non-nil rate limiter")
	}

	if rl.rateLimit != config.RateLimit {
		t.Errorf("Expected rate limit %v, got %v", config.RateLimit, rl.rateLimit)
	}

	if rl.burstSize != config.BurstSize {
		t.Errorf("Expected burst size %d, got %d", config.BurstSize, rl.burstSize)
	}

	if !rl.cleanupEnabled {
		t.Error("Expected cleanup to be enabled")
	}
}

func TestGetLimiter(t *testing.T) {
	config := DefaultRateLimiterConfig()
	rl := NewRateLimiter(config)
	defer rl.Stop()

	limiter1 := rl.GetLimiter("192.168.1.1")
	if limiter1 == nil {
		t.Fatal("Expected non-nil limiter")
	}

	limiter2 := rl.GetLimiter("192.168.1.1")
	if limiter1 != limiter2 {
		t.Error("Expected same limiter instance for same IP")
	}

	limiter3 := rl.GetLimiter("192.168.1.2")
	if limiter1 == limiter3 {
		t.Error("Expected different limiter instance for different IP")
	}
}

func TestRateLimiting(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(2),
		BurstSize:         3,
		CleanupInterval:   0,
		InactiveThreshold: 0,
	}
	rl := NewRateLimiter(config)

	limiter := rl.GetLimiter("192.168.1.1")

	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should have been allowed (within burst)", i+1)
		}
	}

	if limiter.Allow() {
		t.Error("Request 4 should have been denied (burst exhausted)")
	}

	time.Sleep(550 * time.Millisecond)

	if !limiter.Allow() {
		t.Error("Request should have been allowed after waiting")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(2),
		BurstSize:         2,
		CleanupInterval:   0,
		InactiveThreshold: 0,
	}
	rl := NewRateLimiter(config)

	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	if w.Header().Get("Retry-After") != "1" {
		t.Errorf("Expected Retry-After header to be '1', got '%s'", w.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddlewareMultipleIPs(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(1),
		BurstSize:         1,
		CleanupInterval:   0,
		InactiveThreshold: 0,
	}
	rl := NewRateLimiter(config)

	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("IP1 request 1: Expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 request 2: Expected 429, got %d", w2.Code)
	}

	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.2:54321"
	w3 := httptest.NewRecorder()
	handler(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("IP2 request 1: Expected 200, got %d", w3.Code)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		expected   string
	}{
		{
			name:       "Direct connection",
			remoteAddr: "192.168.1.1:12345",
			xff:        "",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "127.0.0.1:12345",
			xff:        "203.0.113.1, 198.51.100.1, 192.0.2.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For with spaces",
			remoteAddr: "127.0.0.1:12345",
			xff:        "  203.0.113.1  , 198.51.100.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			xff:        "",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			ip := extractIP(req)
			if ip != tt.expected {
				t.Errorf("Expected IP %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(10),
		BurstSize:         20,
		CleanupInterval:   100 * time.Millisecond,
		InactiveThreshold: 20,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	rl.GetLimiter("192.168.1.1")
	rl.GetLimiter("192.168.1.2")
	rl.GetLimiter("192.168.1.3")

	rl.mu.RLock()
	initialCount := len(rl.limiters)
	rl.mu.RUnlock()

	if initialCount != 3 {
		t.Fatalf("Expected 3 limiters, got %d", initialCount)
	}

	time.Sleep(250 * time.Millisecond)

	rl.mu.RLock()
	finalCount := len(rl.limiters)
	rl.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", finalCount)
	}
}

func TestCleanupWithActiveUsage(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(10),
		BurstSize:         20,
		CleanupInterval:   100 * time.Millisecond,
		InactiveThreshold: 20,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	limiter := rl.GetLimiter("192.168.1.1")

	for i := 0; i < 20; i++ {
		limiter.Allow()
	}

	time.Sleep(250 * time.Millisecond)

	rl.mu.RLock()
	count := len(rl.limiters)
	rl.mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 active limiter after cleanup, got %d", count)
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := DefaultRateLimiterConfig()
	rl := NewRateLimiter(config)
	defer rl.Stop()

	var wg sync.WaitGroup
	iterations := 100
	goroutines := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "192.168.1." + string(rune('1'+id))
			for j := 0; j < iterations; j++ {
				limiter := rl.GetLimiter(ip)
				limiter.Allow()
			}
		}(i)
	}

	wg.Wait()

	rl.mu.RLock()
	count := len(rl.limiters)
	rl.mu.RUnlock()

	if count == 0 {
		t.Error("Expected at least some limiters to be created")
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	SendError(w, "test error", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test error") {
		t.Errorf("Expected body to contain 'test error', got: %s", body)
	}
	if !strings.Contains(body, `"success":false`) {
		t.Errorf("Expected body to contain success:false, got: %s", body)
	}
}

func TestSendJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{
		"message": "hello",
		"count":   42,
	}

	err := SendJSON(w, data, http.StatusOK)
	if err != nil {
		t.Fatalf("SendJSON returned error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "hello") {
		t.Errorf("Expected body to contain 'hello', got: %s", body)
	}
	if !strings.Contains(body, "42") {
		t.Errorf("Expected body to contain '42', got: %s", body)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	if w.Body.String() != "created" {
		t.Errorf("Expected body 'created', got '%s'", w.Body.String())
	}
}

func TestResponseWriter(t *testing.T) {
	baseWriter := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: baseWriter,
		statusCode:     http.StatusOK,
	}

	rw.WriteHeader(http.StatusNotFound)
	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404, got %d", rw.statusCode)
	}

	if baseWriter.Code != http.StatusNotFound {
		t.Errorf("Expected base writer status 404, got %d", baseWriter.Code)
	}
}

func TestStopRateLimiter(t *testing.T) {
	config := DefaultRateLimiterConfig()
	rl := NewRateLimiter(config)

	rl.Stop()
	rl.Stop()
}

func TestRateLimiterWithoutCleanup(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(10),
		BurstSize:         20,
		CleanupInterval:   0,
		InactiveThreshold: 0,
	}
	rl := NewRateLimiter(config)

	if rl.cleanupEnabled {
		t.Error("Expected cleanup to be disabled")
	}

	rl.Stop()
}

func TestStartCleanup(t *testing.T) {
	config := RateLimiterConfig{
		RateLimit:         rate.Limit(10),
		BurstSize:         20,
		CleanupInterval:   50 * time.Millisecond,
		InactiveThreshold: 20,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl.StartCleanup(ctx, 20)

	rl.GetLimiter("192.168.1.1")
	rl.GetLimiter("192.168.1.2")

	time.Sleep(150 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	rl.mu.RLock()
	count := len(rl.limiters)
	rl.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", count)
	}
}
