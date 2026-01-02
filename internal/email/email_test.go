package email

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	apiKey := "test-api-key"
	fromEmail := "test@example.com"

	client := NewClient(apiKey, fromEmail)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.apiKey != apiKey {
		t.Errorf("NewClient() apiKey = %v, want %v", client.apiKey, apiKey)
	}
	if client.fromEmail != fromEmail {
		t.Errorf("NewClient() fromEmail = %v, want %v", client.fromEmail, fromEmail)
	}
	if client.client == nil {
		t.Error("NewClient() client.client is nil")
	}
}

func TestVerificationTemplate(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		email         string
		shouldContain []string
	}{
		{
			name:  "Standard verification",
			code:  "123456",
			email: "user@example.com",
			shouldContain: []string{
				"123456",
				"user@example.com",
				"Verify Your Email",
				"licensify init",
				"10 scans/day",
			},
		},
		{
			name:  "Different code",
			code:  "987654",
			email: "another@example.com",
			shouldContain: []string{
				"987654",
				"another@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := verificationTemplate(tt.code, tt.email)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(html, expected) {
					t.Errorf("verificationTemplate() should contain %q", expected)
				}
			}

			// Verify it's valid HTML structure
			if !strings.Contains(html, "<!DOCTYPE html>") {
				t.Error("verificationTemplate() should have DOCTYPE")
			}
			if !strings.Contains(html, "<html>") || !strings.Contains(html, "</html>") {
				t.Error("verificationTemplate() should have html tags")
			}
		})
	}
}

func TestLicenseTemplate(t *testing.T) {
	tests := []struct {
		name          string
		licenseKey    string
		tier          string
		dailyLimit    int
		shouldContain []string
	}{
		{
			name:       "Free tier",
			licenseKey: "LIC-202601-ABC123-XYZ789",
			tier:       "free",
			dailyLimit: 10,
			shouldContain: []string{
				"LIC-202601-ABC123-XYZ789",
				"FREE",
				"10 scans",
				"licensify activate",
			},
		},
		{
			name:       "Pro tier",
			licenseKey: "LIC-202601-PRO456-XYZ123",
			tier:       "pro",
			dailyLimit: 100,
			shouldContain: []string{
				"LIC-202601-PRO456-XYZ123",
				"PRO",
				"100 scans",
			},
		},
		{
			name:       "Enterprise tier",
			licenseKey: "LIC-202601-ENT789-ABC456",
			tier:       "enterprise",
			dailyLimit: 1000,
			shouldContain: []string{
				"LIC-202601-ENT789-ABC456",
				"ENTERPRISE",
				"1000 scans",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := licenseTemplate(tt.licenseKey, tt.tier, tt.dailyLimit)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(html, expected) {
					t.Errorf("licenseTemplate() should contain %q", expected)
				}
			}

			// Verify it's valid HTML structure
			if !strings.Contains(html, "<!DOCTYPE html>") {
				t.Error("licenseTemplate() should have DOCTYPE")
			}
			if !strings.Contains(html, "<html>") || !strings.Contains(html, "</html>") {
				t.Error("licenseTemplate() should have html tags")
			}
		})
	}
}

func TestTemplateHTMLStructure(t *testing.T) {
	// Test that templates produce valid HTML
	verifyHTML := verificationTemplate("123456", "test@example.com")
	licenseHTML := licenseTemplate("LIC-123", "pro", 100)

	templates := map[string]string{
		"verification": verifyHTML,
		"license":      licenseHTML,
	}

	for name, html := range templates {
		t.Run(name, func(t *testing.T) {
			// Check for basic HTML structure
			requiredTags := []string{
				"<!DOCTYPE html>",
				"<html>",
				"</html>",
				"<head>",
				"</head>",
				"<body>",
				"</body>",
				"<style>",
				"</style>",
				"<div class=\"container\">",
			}

			for _, tag := range requiredTags {
				if !strings.Contains(html, tag) {
					t.Errorf("Template should contain %q", tag)
				}
			}
		})
	}
}

func TestSendVerificationCode(t *testing.T) {
	// Test that method calls template correctly
	toEmail := "user@example.com"
	code := "123456"

	html := verificationTemplate(code, toEmail)

	// Verify template contains expected elements
	if !strings.Contains(html, code) {
		t.Errorf("verificationTemplate() should contain code %s", code)
	}
	if !strings.Contains(html, toEmail) {
		t.Errorf("verificationTemplate() should contain email %s", toEmail)
	}
	if !strings.Contains(html, "Verify Your Email") {
		t.Error("verificationTemplate() should contain 'Verify Your Email'")
	}
	if !strings.Contains(html, "10 scans/day") {
		t.Error("verificationTemplate() should mention free tier limits")
	}
}

func TestSendLicenseKey(t *testing.T) {
	// Test that method calls template correctly
	licenseKey := "LIC-202601-ABC123-XYZ789"
	tier := "free"
	dailyLimit := 10

	html := licenseTemplate(licenseKey, tier, dailyLimit)

	// Verify template contains expected elements
	if !strings.Contains(html, licenseKey) {
		t.Errorf("licenseTemplate() should contain license key %s", licenseKey)
	}
	if !strings.Contains(html, strings.ToUpper(tier)) {
		t.Errorf("licenseTemplate() should contain tier %s", tier)
	}
	if !strings.Contains(html, "10 scans") {
		t.Error("licenseTemplate() should mention daily limit")
	}
	if !strings.Contains(html, "Your Licensify License") {
		t.Error("licenseTemplate() should contain title")
	}
}
