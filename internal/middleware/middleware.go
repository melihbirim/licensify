package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiters       map[string]*rate.Limiter
	mu             sync.RWMutex
	cleanupTicker  *time.Ticker
	cleanupDone    chan struct{}
	rateLimit      rate.Limit
	burstSize      int
	cleanupEnabled bool
}

type RateLimiterConfig struct {
	RateLimit         rate.Limit
	BurstSize         int
	CleanupInterval   time.Duration
	InactiveThreshold float64
}

func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RateLimit:         rate.Limit(10),
		BurstSize:         20,
		CleanupInterval:   5 * time.Minute,
		InactiveThreshold: 20,
	}
}

func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters:       make(map[string]*rate.Limiter),
		cleanupDone:    make(chan struct{}),
		rateLimit:      config.RateLimit,
		burstSize:      config.BurstSize,
		cleanupEnabled: config.CleanupInterval > 0,
	}

	if rl.cleanupEnabled {
		rl.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go rl.cleanup(config.InactiveThreshold)
	}

	return rl
}

func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.rateLimit, rl.burstSize)
		rl.limiters[ip] = limiter
		rl.mu.Unlock()
	}

	return limiter
}

func (rl *RateLimiter) cleanup(threshold float64) {
	for {
		select {
		case <-rl.cleanupDone:
			return
		case <-rl.cleanupTicker.C:
			rl.mu.Lock()
			for ip, limiter := range rl.limiters {
				if limiter.Tokens() >= threshold {
					delete(rl.limiters, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) Stop() {
	if rl.cleanupEnabled && rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
		select {
		case <-rl.cleanupDone:
			// Already closed
		default:
			close(rl.cleanupDone)
		}
	}
}

func (rl *RateLimiter) StartCleanup(ctx context.Context, threshold float64) {
	if !rl.cleanupEnabled {
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-rl.cleanupDone:
				return
			case <-rl.cleanupTicker.C:
				rl.mu.Lock()
				for ip, limiter := range rl.limiters {
					if limiter.Tokens() >= threshold {
						delete(rl.limiters, ip)
					}
				}
				rl.mu.Unlock()
			}
		}
	}()
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		limiter := rl.GetLimiter(ip)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			SendError(w, "Too many requests from this IP", http.StatusTooManyRequests)
			log.Printf("Rate limit exceeded for IP: %s", ip)
			return
		}

		next(w, r)
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func SendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Success: false,
		Error:   message,
	})
}

func SendJSON(w http.ResponseWriter, data interface{}, code int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(data)
}

func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(rw, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.statusCode, duration)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
