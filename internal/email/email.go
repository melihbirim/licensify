package email

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// ResendAPIURL is the Resend API endpoint
	ResendAPIURL = "https://api.resend.com/emails"
	// DefaultTimeout for HTTP requests
	DefaultTimeout = 10 * time.Second
)

// Client handles email sending via Resend API
type Client struct {
	apiKey    string
	fromEmail string
	client    *http.Client
}

// NewClient creates a new email client
func NewClient(apiKey, fromEmail string) *Client {
	return &Client{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		client:    &http.Client{Timeout: DefaultTimeout},
	}
}

// SendVerificationCode sends a verification code email
func (c *Client) SendVerificationCode(toEmail, code string) error {
	html := verificationTemplate(code, toEmail)
	return c.send(toEmail, "Verify Your Email - Licensify", html)
}

// SendLicenseKey sends a license key email
func (c *Client) SendLicenseKey(toEmail, licenseKey, tier string, dailyLimit int) error {
	html := licenseTemplate(licenseKey, tier, dailyLimit)
	return c.send(toEmail, "Your Licensify License Key", html)
}

// send sends an email via Resend API
func (c *Client) send(toEmail, subject, html string) error {
	payload := map[string]interface{}{
		"from":    c.fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    html,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", ResendAPIURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error (status %d): %s", resp.StatusCode, body)
	}

	return nil
}

// verificationTemplate generates HTML for verification email
func verificationTemplate(code, email string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        .code { 
            font-size: 32px; 
            font-weight: bold; 
            letter-spacing: 8px; 
            text-align: center;
            background: #f5f5f5;
            padding: 20px;
            border-radius: 8px;
            margin: 30px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧾 Verify Your Email</h1>
        <p>Your verification code is:</p>
        <div class="code">%s</div>
        <p>Run: <code>licensify init --email=%s --verify=%s</code></p>
        <p><strong>Free Tier: 10 scans/day</strong></p>
    </div>
</body>
</html>`, code, email, code)
}

// licenseTemplate generates HTML for license key email
func licenseTemplate(licenseKey, tier string, dailyLimit int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        .license-key {
            font-size: 18px;
            font-weight: bold;
            font-family: monospace;
            background: #f0f9ff;
            padding: 20px;
            border-radius: 8px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎉 Your Licensify License</h1>
        <p>Your license key:</p>
        <div class="license-key">%s</div>
        <p><strong>Tier:</strong> %s | <strong>Daily Limit:</strong> %d scans</p>
        <p>Quick start: <code>licensify activate %s</code></p>
    </div>
</body>
</html>`, licenseKey, strings.ToUpper(tier), dailyLimit, licenseKey)
}
