package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// getHardwareID returns a unique hardware identifier for the current machine
func getHardwareID() (string, error) {
	var id string
	var err error

	switch runtime.GOOS {
	case "darwin":
		id, err = getMacOSHardwareID()
	case "linux":
		id, err = getLinuxHardwareID()
	case "windows":
		id, err = getWindowsHardwareID()
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err != nil {
		return "", err
	}

	// Hash the ID to get consistent 64-character hex string (SHA256)
	hash := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%x", hash), nil
}

func getMacOSHardwareID() (string, error) {
	// Try to get system serial number
	cmd := exec.Command("ioreg", "-l")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get system info: %w", err)
	}

	// Look for IOPlatformSerialNumber
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "IOPlatformSerialNumber") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				serial := strings.TrimSpace(strings.Trim(parts[1], `"`))
				if serial != "" {
					return serial, nil
				}
			}
		}
	}

	// Fallback to system UUID
	cmd = exec.Command("system_profiler", "SPHardwareDataType")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get hardware UUID: %w", err)
	}

	lines = strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Hardware UUID") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				uuid := strings.TrimSpace(parts[1])
				if uuid != "" {
					return uuid, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not determine hardware ID")
}

func getLinuxHardwareID() (string, error) {
	// Try /etc/machine-id first
	data, err := os.ReadFile("/etc/machine-id")
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	// Try /var/lib/dbus/machine-id
	data, err = os.ReadFile("/var/lib/dbus/machine-id")
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	// Try DMI product UUID
	data, err = os.ReadFile("/sys/class/dmi/id/product_uuid")
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not determine hardware ID")
}

func getWindowsHardwareID() (string, error) {
	// Use WMIC to get UUID
	cmd := exec.Command("wmic", "csproduct", "get", "UUID")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get system UUID: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected wmic output")
	}

	uuid := strings.TrimSpace(lines[1])
	if uuid == "" {
		return "", fmt.Errorf("empty UUID from wmic")
	}

	return uuid, nil
}
