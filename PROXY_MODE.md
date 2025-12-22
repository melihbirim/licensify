# Proxy Mode - API Key Protection

Proxy mode is an enhanced security feature where Licensify acts as a transparent proxy between your clients and external API providers (OpenAI, Anthropic, etc.). This ensures that **clients never receive the actual API keys**.

## Why Use Proxy Mode?

### Security Benefits
- 🔒 **API keys never leave the server** - clients can't extract or abuse them
- 📊 **Server-side usage tracking** - accurate monitoring of API calls
- 🚦 **Server-side rate limiting** - enforce limits reliably
- 🔄 **Easy key rotation** - update keys without client updates
- 🎯 **Multi-tenant support** - one key serves many customers

### Use Cases
1. **Desktop/Mobile Apps** - Protect OpenAI/Anthropic keys in distributed apps
2. **Browser Extensions** - Secure API access without exposing keys
3. **CLI Tools** - Centralized key management
4. **Enterprise SaaS** - Serve multiple customers with one API key

## How It Works

```
┌─────────┐         ┌────────────┐         ┌─────────────┐
│ Client  │ License │ Licensify  │ API Key │   OpenAI    │
│   App   ├────────>│   Proxy    ├────────>│  Anthropic  │
└─────────┘         └────────────┘         └─────────────┘
   Never sees          Validates              Real API
   API key             & forwards             interaction
```

1. Client sends request to Licensify with their license key
2. Licensify validates license and checks rate limits
3. Request is forwarded to actual API (OpenAI, Anthropic, etc.)
4. Response is streamed back to client
5. Usage is tracked server-side

## Configuration

### Enable Proxy Mode

```bash
# .env
PROXY_MODE=true
OPENAI_API_KEY=sk-proj-...
ANTHROPIC_API_KEY=sk-ant-...
```

### Supported Providers

| Provider | Endpoint | API Key Env Var |
|----------|----------|----------------|
| OpenAI | `/proxy/openai/*` | `OPENAI_API_KEY` |
| Anthropic | `/proxy/anthropic/*` | `ANTHROPIC_API_KEY` |

## API Usage

### Request Format

All proxy requests use the same format:

```bash
POST /proxy/{provider}/{api_path}
Content-Type: application/json

{
  "license_key": "LIC-xxx",
  "hardware_id": "hw-123",
  "provider": "openai|anthropic",
  "body": {
    // Original API request body
  }
}
```

### OpenAI Example

```bash
curl -X POST "http://localhost:8080/proxy/openai/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "LIC-abc123",
    "hardware_id": "hw-macbook-pro",
    "provider": "openai",
    "body": {
      "model": "gpt-3.5-turbo",
      "messages": [
        {"role": "user", "content": "Hello!"}
      ]
    }
  }'
```

### Anthropic Example

```bash
curl -X POST "http://localhost:8080/proxy/anthropic/v1/messages" \
  -H "Content-Type: application/json" \
  -d '{
    "license_key": "LIC-abc123",
    "hardware_id": "hw-macbook-pro",
    "provider": "anthropic",
    "body": {
      "model": "claude-3-haiku-20240307",
      "max_tokens": 1024,
      "messages": [
        {"role": "user", "content": "Hello!"}
      ]
    }
  }'
```

## Rate Limiting

Proxy mode enforces tier-based rate limits server-side:

### Rate Limit Headers

Every response includes:
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 2025-12-24T00:00:00Z
```

### Rate Limit Exceeded

```json
HTTP 429 Too Many Requests

{
  "error": {
    "message": "Daily limit of 1000 requests exceeded. Current usage: 1000",
    "type": "rate_limit_exceeded",
    "code": "rate_limit_exceeded"
  }
}
```

## Client Library Examples

### Python Client

```python
import requests

class LicensifyProxy:
    def __init__(self, server_url, license_key, hardware_id):
        self.server_url = server_url
        self.license_key = license_key
        self.hardware_id = hardware_id
    
    def openai_chat(self, messages, model="gpt-3.5-turbo"):
        response = requests.post(
            f"{self.server_url}/proxy/openai/v1/chat/completions",
            json={
                "license_key": self.license_key,
                "hardware_id": self.hardware_id,
                "provider": "openai",
                "body": {
                    "model": model,
                    "messages": messages
                }
            }
        )
        return response.json()

# Usage
client = LicensifyProxy("https://your-server.com", "LIC-xxx", "hw-123")
result = client.openai_chat([{"role": "user", "content": "Hello!"}])
print(result)
```

### JavaScript/TypeScript

```typescript
class LicensifyProxy {
  constructor(
    private serverUrl: string,
    private licenseKey: string,
    private hardwareId: string
  ) {}

  async openaiChat(messages: any[], model = "gpt-3.5-turbo") {
    const response = await fetch(
      `${this.serverUrl}/proxy/openai/v1/chat/completions`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          license_key: this.licenseKey,
          hardware_id: this.hardwareId,
          provider: "openai",
          body: { model, messages }
        })
      }
    );
    return response.json();
  }
}

// Usage
const client = new LicensifyProxy(
  "https://your-server.com",
  "LIC-xxx",
  "hw-123"
);
const result = await client.openaiChat([
  { role: "user", content: "Hello!" }
]);
```

## Testing

Run the included test script:

```bash
# Update LICENSE_KEY and HARDWARE_ID in test-proxy.sh
./test-proxy.sh
```

Or test manually:

```bash
# 1. Start server with proxy mode
PROXY_MODE=true \
OPENAI_API_KEY=sk-... \
./licensify

# 2. Create a license (using /init + /verify)
# 3. Activate the license
# 4. Test proxy endpoint (see examples above)
```

## Comparison: Direct vs Proxy Mode

| Feature | Direct Mode | Proxy Mode |
|---------|-------------|------------|
| Client has API key | ✅ Yes (encrypted) | ❌ No |
| Key can be extracted | ⚠️ Possible | ✅ Impossible |
| Rate limiting | Client-side | ✅ Server-side |
| Usage tracking | Honor system | ✅ Accurate |
| Key rotation | Requires client update | ✅ Server-only |
| Latency | Lower | Slightly higher |
| Offline usage | ✅ Yes | ❌ No |

## Security Considerations

### ✅ Advantages
- API keys are never exposed to clients
- Server-side enforcement of all limits
- Centralized audit logging
- Easy key rotation and revocation

### ⚠️ Considerations
- Server becomes a critical dependency
- Additional latency (extra hop)
- Server bandwidth scales with usage
- Requires reliable hosting

## Production Deployment

For production proxy mode:

1. **Use PostgreSQL** for reliable usage tracking
2. **Enable HTTPS** - never run proxy over HTTP
3. **Monitor rate limits** - set up alerts
4. **Cache responses** - consider caching for identical requests
5. **Load balancing** - scale horizontally for high traffic
6. **Backup keys** - store API keys securely (use secrets management)

### Environment Variables

```bash
PROXY_MODE=true
DATABASE_URL=postgres://...  # Use PostgreSQL for production
OPENAI_API_KEY=sk-...        # Store in secrets manager
ANTHROPIC_API_KEY=sk-ant-... # Store in secrets manager
```

## Future Enhancements

Planned features for proxy mode:

- [ ] Response caching (identical requests)
- [ ] Request queuing (rate smoothing)
- [ ] Multi-region routing (geo-based)
- [ ] Custom endpoints (self-hosted LLMs)
- [ ] WebSocket streaming support
- [ ] Cost tracking (token usage)
- [ ] Analytics dashboard

## Troubleshooting

### Proxy endpoint returns 503

```json
{"error": "OpenAI API key not configured"}
```

**Solution**: Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` in environment variables.

### Rate limit exceeded

```json
{"error": {"type": "rate_limit_exceeded"}}
```

**Solution**: Wait until reset time or upgrade tier. Check `X-RateLimit-Reset` header.

### Hardware ID not activated

```json
{"error": "Hardware ID not activated for this license"}
```

**Solution**: Run activation first: `POST /activate` with license_key and hardware_id.

## Support

- GitHub Issues: https://github.com/melihbirim/licensify/issues
- Documentation: https://github.com/melihbirim/licensify
- Feature Requests: Use issue label `proxy-mode`
