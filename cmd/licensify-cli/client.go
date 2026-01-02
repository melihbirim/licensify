package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func newHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *HTTPClient) post(endpoint string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error message
		var errorResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errorResp) == nil {
			if errorResp.Error != "" {
				return nil, fmt.Errorf("API error: %s", errorResp.Error)
			}
			if errorResp.Message != "" {
				return nil, fmt.Errorf("API error: %s", errorResp.Message)
			}
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Init requests a new license
type InitRequest struct {
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

type InitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email"`
}

func (c *HTTPClient) requestLicense(email, tier string) (*InitResponse, error) {
	body, err := c.post("/init", InitRequest{
		Email: email,
		Tier:  tier,
	})
	if err != nil {
		return nil, err
	}

	var resp InitResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Verify verifies email and creates license
type VerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
	Tier  string `json:"tier"`
}

type VerifyResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	LicenseKey   string    `json:"license_key"`
	Tier         string    `json:"tier"`
	ExpiresAt    time.Time `json:"expires_at"`
	DailyLimit   int       `json:"daily_limit"`
	MonthlyLimit int       `json:"monthly_limit"`
}

func (c *HTTPClient) verifyEmail(email, code, tier string) (*VerifyResponse, error) {
	body, err := c.post("/verify", VerifyRequest{
		Email: email,
		Code:  code,
		Tier:  tier,
	})
	if err != nil {
		return nil, err
	}

	var resp VerifyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Activate activates a license
type ActivateRequest struct {
	LicenseKey string `json:"license_key"`
	HardwareID string `json:"hardware_id"`
}

type ActivateResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	EncryptedBundle string `json:"encrypted_bundle,omitempty"`
	BundleSignature string `json:"bundle_signature,omitempty"`
	ProxyKey        string `json:"proxy_key,omitempty"`
}

func (c *HTTPClient) activateLicense(licenseKey, hardwareID string) (*ActivateResponse, error) {
	body, err := c.post("/activate", ActivateRequest{
		LicenseKey: licenseKey,
		HardwareID: hardwareID,
	})
	if err != nil {
		return nil, err
	}

	var resp ActivateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Check checks license status
type CheckRequest struct {
	LicenseKey string `json:"license_key"`
}

type CheckResponse struct {
	Valid        bool      `json:"valid"`
	CustomerName string    `json:"customer_name,omitempty"`
	Tier         string    `json:"tier,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	DailyUsage   int       `json:"daily_usage,omitempty"`
	MonthlyUsage int       `json:"monthly_usage,omitempty"`
	DailyLimit   int       `json:"daily_limit,omitempty"`
	MonthlyLimit int       `json:"monthly_limit,omitempty"`
}

func (c *HTTPClient) checkLicense(licenseKey string) (*CheckResponse, error) {
	body, err := c.post("/check", CheckRequest{
		LicenseKey: licenseKey,
	})
	if err != nil {
		return nil, err
	}

	var resp CheckResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}
