#!/bin/bash
# Test script for proxy mode
# Usage: ./test-proxy.sh

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVER_URL="http://localhost:8080"
LICENSE_KEY="LIC-xxx"  # Replace with actual license key
HARDWARE_ID="test-hw-123"

echo -e "${YELLOW}Testing Licensify Proxy Mode${NC}\n"

# Test 1: OpenAI Proxy
echo -e "${YELLOW}Test 1: OpenAI Chat Completion via Proxy${NC}"
curl -X POST "${SERVER_URL}/proxy/openai/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "'"${LICENSE_KEY}"'",
    "hardware_id": "'"${HARDWARE_ID}"'",
    "provider": "openai",
    "body": {
      "model": "gpt-3.5-turbo",
      "messages": [
        {"role": "user", "content": "Say hello in JSON format"}
      ],
      "max_tokens": 50
    }
  }'
echo -e "\n"

# Test 2: Anthropic Proxy
echo -e "${YELLOW}Test 2: Anthropic Messages via Proxy${NC}"
curl -X POST "${SERVER_URL}/proxy/anthropic/v1/messages" \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "'"${LICENSE_KEY}"'",
    "hardware_id": "'"${HARDWARE_ID}"'",
    "provider": "anthropic",
    "body": {
      "model": "claude-3-haiku-20240307",
      "max_tokens": 50,
      "messages": [
        {"role": "user", "content": "Say hello"}
      ]
    }
  }'
echo -e "\n"

# Test 3: Rate Limit Check
echo -e "${YELLOW}Test 3: Check Rate Limit Headers${NC}"
curl -v -X POST "${SERVER_URL}/proxy/openai/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "'"${LICENSE_KEY}"'",
    "hardware_id": "'"${HARDWARE_ID}"'",
    "provider": "openai",
    "body": {
      "model": "gpt-3.5-turbo",
      "messages": [{"role": "user", "content": "hi"}],
      "max_tokens": 10
    }
  }' 2>&1 | grep -i "x-ratelimit"
echo -e "\n"

echo -e "${GREEN}Proxy tests completed!${NC}"
