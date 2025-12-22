#!/bin/bash

echo "🧪 Manual Proxy Key Test"
echo ""

# Step 1: Init with email
echo "1️⃣ Initiating license request..."
EMAIL="test@birim.one"
curl -s -X POST http://localhost:8080/init \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\"}" | jq '.'
echo ""

# Step 2: Verify and get license
echo "2️⃣ Getting verification code from database..."
sleep 1  # Wait for DB write
VERIFY_CODE=$(sqlite3 licensify-proxy-test.db "SELECT code FROM verification_codes WHERE email='$EMAIL' ORDER BY created_at DESC LIMIT 1")
echo "Verification code: $VERIFY_CODE"
echo ""

echo "3️⃣ Verifying and creating license..."
VERIFY_RESP=$(curl -s -X POST http://localhost:8080/verify \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"code\":\"$VERIFY_CODE\",\"customer_name\":\"Proxy Test\",\"tier\":\"FREE\"}")

echo "$VERIFY_RESP" | jq '.'
LICENSE=$(echo "$VERIFY_RESP" | jq -r '.license_key')
echo "License: $LICENSE"
echo ""

# Step 3: Activate
echo "4️⃣ Activating license (proxy mode should return proxy key)..."
HARDWARE="test-$(date +%s)"
ACTIVATION=$(curl -s -X POST http://localhost:8080/activate \
  -H "Content-Type: application/json" \
  -d "{\"license_key\":\"$LICENSE\",\"hardware_id\":\"$HARDWARE\"}")

echo "$ACTIVATION" | jq '.'
echo ""

# Step 4: Get proxy key from database
echo "5️⃣ Checking proxy key in database..."
sqlite3 licensify-proxy-test.db "SELECT proxy_key, license_id, hardware_id FROM proxy_keys WHERE license_id = '$LICENSE'" | head -1
PROXY_KEY=$(sqlite3 licensify-proxy-test.db "SELECT proxy_key FROM proxy_keys WHERE license_id = '$LICENSE'")
echo "Proxy key: ${PROXY_KEY:0:20}..."
echo ""

# Step 5: Test proxy with valid key
echo "6️⃣ Testing proxy endpoint with proxy key..."
curl -s -X POST http://localhost:8080/proxy/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{
    \"proxy_key\": \"$PROXY_KEY\",
    \"provider\": \"openai\",
    \"body\": {
      \"model\": \"gpt-3.5-turbo\",
      \"messages\": [{\"role\": \"user\", \"content\": \"Say 'test'\"}],
      \"max_tokens\": 5
    }
  }" | jq '.'
echo ""

# Step 6: Test with invalid proxy key
echo "7️⃣ Testing with invalid proxy key (should fail)..."
curl -s -X POST http://localhost:8080/proxy/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "proxy_key": "px_invalid_key_1234567890123456789012345678",
    "provider": "openai",
    "body": {
      "model": "gpt-3.5-turbo",
      "messages": [{"role": "user", "content": "Hi"}],
      "max_tokens": 5
    }
  }' | jq '.'

echo ""
echo "✅ Test complete!"
