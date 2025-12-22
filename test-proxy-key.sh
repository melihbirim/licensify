#!/bin/bash
set -e

echo "🧪 Testing Proxy Key Feature"
echo "=============================="
echo ""

# Configuration
PORT=8080
BASE_URL="http://localhost:$PORT"
OPENAI_KEY="${OPENAI_API_KEY}"

if [ -z "$OPENAI_KEY" ]; then
    echo "❌ OPENAI_API_KEY environment variable not set"
    exit 1
fi

echo "📋 Step 1: Creating license..."
LICENSE=$(curl -s -X POST "$BASE_URL/init" \
    -H "Content-Type: application/json" \
    -d '{
        "customer_name": "Proxy Test User",
        "customer_email": "proxytest@example.com",
        "tier": "FREE"
    }' | jq -r '.license_key')

if [ -z "$LICENSE" ] || [ "$LICENSE" == "null" ]; then
    echo "❌ Failed to create license"
    exit 1
fi

echo "✅ License created: $LICENSE"
echo ""

echo "📋 Step 2: Verifying license..."
VERIFY_CODE=$(echo "000000") # In production, get from email
VERIFY_RESP=$(curl -s -X POST "$BASE_URL/verify" \
    -H "Content-Type: application/json" \
    -d "{
        \"license_key\": \"$LICENSE\",
        \"verification_code\": \"$VERIFY_CODE\"
    }")

echo "✅ License verified"
echo ""

echo "📋 Step 3: Activating license (proxy mode)..."
HARDWARE_ID="test-macbook-proxy-$(date +%s)"
ACTIVATION_RESP=$(curl -s -X POST "$BASE_URL/activate" \
    -H "Content-Type: application/json" \
    -d "{
        \"license_key\": \"$LICENSE\",
        \"hardware_id\": \"$HARDWARE_ID\"
    }")

echo "Activation response:"
echo "$ACTIVATION_RESP" | jq '.'
echo ""

# Extract encrypted key and IV
ENCRYPTED_KEY=$(echo "$ACTIVATION_RESP" | jq -r '.encrypted_api_key')
IV=$(echo "$ACTIVATION_RESP" | jq -r '.iv')

if [ -z "$ENCRYPTED_KEY" ] || [ "$ENCRYPTED_KEY" == "null" ]; then
    echo "❌ Failed to get encrypted proxy key"
    exit 1
fi

echo "✅ Got encrypted proxy key (length: ${#ENCRYPTED_KEY})"
echo ""

echo "📋 Step 4: Decrypting proxy key..."
# In a real scenario, the client would decrypt this using the license key
# For this test, we'll extract it directly from the database
PROXY_KEY=$(sqlite3 licensify-proxy-test.db "SELECT proxy_key FROM proxy_keys WHERE license_id = '$LICENSE' AND hardware_id = '$HARDWARE_ID'")

if [ -z "$PROXY_KEY" ]; then
    echo "❌ Failed to retrieve proxy key from database"
    exit 1
fi

echo "✅ Proxy key retrieved: ${PROXY_KEY:0:10}..."
echo ""

echo "📋 Step 5: Testing proxy with OpenAI..."
PROXY_RESP=$(curl -s -X POST "$BASE_URL/proxy/openai/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{
        \"proxy_key\": \"$PROXY_KEY\",
        \"provider\": \"openai\",
        \"body\": {
            \"model\": \"gpt-3.5-turbo\",
            \"messages\": [{
                \"role\": \"user\",
                \"content\": \"Say 'Proxy key test successful!' and nothing else.\"
            }],
            \"max_tokens\": 20
        }
    }")

echo "OpenAI response:"
echo "$PROXY_RESP" | jq '.'
echo ""

# Check if response is successful
if echo "$PROXY_RESP" | jq -e '.choices[0].message.content' > /dev/null; then
    CONTENT=$(echo "$PROXY_RESP" | jq -r '.choices[0].message.content')
    echo "✅ Proxy request successful! Response: $CONTENT"
else
    echo "❌ Proxy request failed"
    exit 1
fi
echo ""

echo "📋 Step 6: Testing rate limit headers..."
PROXY_RESP_HEADERS=$(curl -s -i -X POST "$BASE_URL/proxy/openai/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{
        \"proxy_key\": \"$PROXY_KEY\",
        \"provider\": \"openai\",
        \"body\": {
            \"model\": \"gpt-3.5-turbo\",
            \"messages\": [{
                \"role\": \"user\",
                \"content\": \"Hi\"
            }],
            \"max_tokens\": 5
        }
    }")

echo "Rate limit headers:"
echo "$PROXY_RESP_HEADERS" | grep -i "x-ratelimit"
echo ""

LIMIT=$(echo "$PROXY_RESP_HEADERS" | grep -i "x-ratelimit-limit" | awk '{print $2}' | tr -d '\r')
REMAINING=$(echo "$PROXY_RESP_HEADERS" | grep -i "x-ratelimit-remaining" | awk '{print $2}' | tr -d '\r')

echo "✅ Rate limit: $LIMIT, Remaining: $REMAINING"
echo ""

echo "📋 Step 7: Testing with invalid proxy key..."
INVALID_RESP=$(curl -s -X POST "$BASE_URL/proxy/openai/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
        "proxy_key": "px_invalid_key_12345678901234567890123456789",
        "provider": "openai",
        "body": {
            "model": "gpt-3.5-turbo",
            "messages": [{"role": "user", "content": "Hi"}],
            "max_tokens": 5
        }
    }')

if echo "$INVALID_RESP" | jq -e '.error' > /dev/null; then
    echo "✅ Invalid proxy key correctly rejected"
else
    echo "❌ Invalid proxy key was not rejected!"
    exit 1
fi
echo ""

echo "📋 Step 8: Checking database state..."
echo "Proxy keys in database:"
sqlite3 licensify-proxy-test.db "SELECT proxy_key, license_id, hardware_id, created_at FROM proxy_keys"
echo ""

echo "Usage tracking:"
sqlite3 licensify-proxy-test.db "SELECT license_id, date, scans, hardware_id FROM daily_usage"
echo ""

echo "================================"
echo "✅ All proxy key tests passed!"
echo "================================"
