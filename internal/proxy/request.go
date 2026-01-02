package proxy

import (
"crypto/hmac"
"crypto/sha256"
"encoding/hex"
"encoding/json"
"fmt"
"time"
)

type Request struct {
	ProxyKey  string          `json:"proxy_key"`
	Provider  string          `json:"provider"`
	Body      json.RawMessage `json:"body"`
	Signature string          `json:"signature"`
	Timestamp int64           `json:"timestamp"`
}

func ValidateSignature(proxyKey, provider string, body []byte, timestamp int64, signature string) bool {
	now := time.Now().Unix()
	if abs(now-timestamp) > 300 {
		return false
	}

	message := fmt.Sprintf("%d%s%s", timestamp, provider, string(body))
	h := hmac.New(sha256.New, []byte(proxyKey))
	h.Write([]byte(message))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func redactKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:7] + "..."
}
