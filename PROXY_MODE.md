# Proxy Mode - API Key Protection

Proxy mode is an enhanced security feature where Licensify acts as a transparent proxy between your clients and external API providers (OpenAI, Anthropic, etc.). Clients receive a **generated proxy key** instead of the actual API keys, which remain securely on the server.

## Why Use Proxy Mode?

### Security Benefits
- 🔒 **API keys never leave the server** - clients receive generated proxy keys instead
- 🔑 **Unique proxy keys per client** - each activation gets its own secure key
- 📊 **Server-side usage tracking** - accurate monitoring of API calls
- 🚦 **Server-side rate limiting** - enforce limits reliably
- 🔄 **Easy key rotation** - update real API keys without client updates
- 🎯 **Multi-tenant support** - one API key serves many customers

### Use Cases
1. **Desktop/Mobile Apps** - Protect OpenAI/Anthropic keys in distributed apps
2. **Browser Extensions** - Secure API access without exposing keys
3. **CLI Tools** - Centralized key management
4. **Enterprise SaaS** - Serve multiple customers with one API key

## How It Works

```
┌─────────┐  Activate  ┌────────────┐              ┌─────────────┐
│ Client  ├───────────>│ Licensify  │              │   OpenAI    │
│   App   │<───────────┤   Server   │              │  Anthropic  │
└────┬────┘ Proxy Key  └──────┬─────┘              └─────────────┘
     │                         │                            ▲
     │   Use Proxy Key         │    Real API Key           │
     └────────────────────────>│───────────────────────────┘
                               │  Validates & forwards
```

1. **Activation**: Client activates license, receives encrypted proxy key (`px_xxx`)
2. **Proxy Request**: Client sends requests with proxy key
3. **Validation**: Licensify validates proxy key and checks rate limits
4. **Forwarding**: Request is forwarded to actual API with real API key
5. **Response**: Response is streamed back to client
6. **Tracking**: Usage is tracked server-side

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

### Step 1: Activate License

When proxy mode is enabled, activation returns an encrypted proxy key instead of the real API key:

```bash
POST /activate
Content-Type: application/json

{
  "license_key": "LIC-xxx",
  "hardware_id": "hw-123"
}
```

Response includes encrypted proxy key:
```json
{
  "success": true,
  "encrypted_api_key": "...",  // This is the proxy key, NOT the real API key
  "iv": "..."
}
```

### Step 2: Make Proxy Requests

All proxy requests use the proxy key:

```bash
POST /proxy/{provider}/{api_path}
Content-Type: application/json

{
  "proxy_key": "px_xxx...",  // Use the decrypted proxy key
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
    "proxy_key": "px_abc123xyz...",
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
    "proxy_key": "px_abc123xyz...",
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
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend
import base64

class LicensifyProxy:
    def __init__(self, server_url):
        self.server_url = server_url
        self.proxy_key = None
    
    def activate(self, license_key, hardware_id):
        """Activate license and get proxy key"""
        response = requests.post(
            f"{self.server_url}/activate",
            json={
                "license_key": license_key,
                "hardware_id": hardware_id
            }
        )
        data = response.json()
        
        # Decrypt the proxy key
        encrypted = base64.b64decode(data["encrypted_api_key"])
        iv = base64.b64decode(data["iv"])
        
        # Use license_key as decryption key (first 32 bytes)
        key = license_key.encode()[:32].ljust(32, b'\0')
        
        cipher = Cipher(algorithms.AES(key), modes.CBC(iv), backend=default_backend())
        decryptor = cipher.decryptor()
        decrypted = decryptor.update(encrypted) + decryptor.finalize()
        
        # Remove padding
        padding_len = decrypted[-1]
        self.proxy_key = decrypted[:-padding_len].decode()
        
        return data
    
    def openai_chat(self, messages, model="gpt-3.5-turbo"):
        """Make OpenAI chat completion request via proxy"""
        if not self.proxy_key:
            raise ValueError("Must activate license first")
        
        response = requests.post(
            f"{self.server_url}/proxy/openai/v1/chat/completions",
            json={
                "proxy_key": self.proxy_key,
                "provider": "openai",
                "body": {
                    "model": model,
                    "messages": messages
                }
            }
        )
        return response.json()

# Usage
client = LicensifyProxy("https://your-server.com")
client.activate("LIC-xxx", "hw-123")
result = client.openai_chat([{"role": "user", "content": "Hello!"}])
print(result)
```

### JavaScript/TypeScript

```typescript
import crypto from 'crypto';

class LicensifyProxy {
  private proxyKey: string | null = null;

  constructor(private serverUrl: string) {}

  async activate(licenseKey: string, hardwareId: string) {
    const response = await fetch(`${this.serverUrl}/activate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ license_key: licenseKey, hardware_id: hardwareId })
    });
    
    const data = await response.json();
    
    // Decrypt the proxy key
    const encrypted = Buffer.from(data.encrypted_api_key, 'base64');
    const iv = Buffer.from(data.iv, 'base64');
    
    // Use license_key as decryption key (first 32 bytes)
    const key = Buffer.alloc(32);
    Buffer.from(licenseKey).copy(key);
    
    const decipher = crypto.createDecipheriv('aes-256-cbc', key, iv);
    let decrypted = decipher.update(encrypted);
    decrypted = Buffer.concat([decrypted, decipher.final()]);
    
    // Remove padding
    const paddingLen = decrypted[decrypted.length - 1];
    this.proxyKey = decrypted.slice(0, -paddingLen).toString();
    
    return data;
  }

  async openaiChat(messages: any[], model = "gpt-3.5-turbo") {
    if (!this.proxyKey) {
      throw new Error("Must activate license first");
    }

    const response = await fetch(
      `${this.serverUrl}/proxy/openai/v1/chat/completions`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          proxy_key: this.proxyKey,
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
