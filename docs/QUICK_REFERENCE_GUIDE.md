# LLM Gateway - Quick Reference Guide

**Enterprise-Grade LLM Management Platform**

---

## 🚀 Quick Start (5 minutes)

### 1. Login
```
URL: https://your-domain.com/login
Default: admin@llm-gateway.local / admin123
```

### 2. Create First API Key
- Navigate to **API Keys** → **+ Generate New Key**
- Name: "My First Key"
- Copy the secret (shown only once!)
- Store safely in environment variables

### 3. Make First Request
```bash
curl -X POST https://api.llm-gateway.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## 📋 Common Tasks

### Create a User Group
1. **Groups & Teams** → **+ Create Group**
2. Enter Name and Description
3. Click **Create**
4. Invite members via email

### Add Team Member
1. Open group
2. **Members** tab → **+ Add Member**
3. Enter email, select role
4. Click **Add**

### Test Chat Interface
1. **Chat** section
2. Select model from dropdown
3. Type message
4. Press Enter or click Send

### Configure Provider
1. **Providers** → **+ Add Provider**
2. Choose type (OpenAI, Azure, etc.)
3. Enter API credentials
4. Click **Test Connection**
5. Click **Add Provider**

### Set Cost Rules
1. **Costs** → **Pricing Rules**
2. For each model, set:
   - Input token price $/1k
   - Output token price $/1k
3. Click **Save**

### Create Prompt Template
1. **Prompts** → **+ Create Template**
2. Enter template name and content
3. Add variables with {curly_braces}
4. Click **Create**
5. Test with data

### Create Smart Route
1. **Routes** → **+ Create Route**
2. Name: "Production GPT-4"
3. Select primary provider
4. Add fallback providers
5. Set retry policy
6. Click **Save**

---

## 🔑 API Key Management

### Generate New Key
```bash
POST /api/keys
Authorization: Bearer ADMIN_KEY

{
  "name": "Production API",
  "team_id": "team-123",
  "rate_limit": 1000
}
```

### Rotate Key
```bash
POST /api/keys/{key-id}/rotate
Authorization: Bearer API_KEY
```

### List Keys
```bash
GET /api/keys
Authorization: Bearer API_KEY
```

### Delete Key
```bash
DELETE /api/keys/{key-id}
Authorization: Bearer ADMIN_KEY
```

---

## 💬 Chat API Examples

### Python
```python
import openai

openai.api_base = "https://api.llm-gateway.com/v1"
openai.api_key = "your-api-key"

response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello!"}],
    temperature=0.7
)

print(response.choices[0].message.content)
```

### JavaScript/Node.js
```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "your-api-key",
  baseURL: "https://api.llm-gateway.com/v1",
});

const message = await client.chat.completions.create({
  model: "gpt-4",
  messages: [{ role: "user", content: "Hello!" }],
  temperature: 0.7,
});

console.log(message.choices[0].message.content);
```

### cURL
```bash
curl -X POST https://api.llm-gateway.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "What is AI?"}],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

---

## 📊 Analytics Queries

### Usage This Month
- **Dashboard** → Scroll to "Usage Statistics"
- Shows: Total requests, tokens consumed, active models
- Update frequency: Real-time

### Cost Breakdown
- **Costs** → **Analytics**
- Filter by: Date range, team, model, provider
- Export CSV for reporting

### Request History
- **Analytics** → **Requests**
- Search by: Request ID, API key, model, status
- View detailed latency and token count per request

### Team Analytics
- **Groups & Teams** → Select group
- View: Member usage, team costs, top models used

---

## 🛠️ Configuration Quick Reference

### Model Parameters
| Parameter | Range | Effect |
|-----------|-------|--------|
| temperature | 0-2 | 0=deterministic, 2=very random |
| max_tokens | 1-model max | Max response length |
| top_p | 0-1 | Diversity (1=all, 0.8=80% probability mass) |
| frequency_penalty | -2 to 2 | Reduce repetition (0=none, 2=max) |

### Rate Limits
- **Per API Key:** 100-1000 RPM (configurable)
- **Per IP:** Configurable (default: 10,000 RPM)
- **Per User:** Configurable (default: unlimited)
- **Response:** `429 Too Many Requests` with `Retry-After` header

### Timeouts
- **Default:** 30 seconds per request
- **Chat requests:** Can stream, no timeout on connection
- **Batch requests:** 60 seconds

### Pricing Models
- **Per-token:** Most accurate, based on actual usage
- **Per-request:** Flat fee per request
- **Subscription:** Monthly flat rate
- **Hybrid:** Combination of above

---

## 🔐 Security Checklists

### On Day 1
- [ ] Change default admin password
- [ ] Create service account for each app
- [ ] Enable audit logging
- [ ] Configure MFA for admins
- [ ] Set rate limits

### On Day 30
- [ ] Rotate all API keys
- [ ] Review access logs
- [ ] Update provider credentials
- [ ] Test backup/recovery
- [ ] Review cost trends

### Quarterly
- [ ] Full security audit
- [ ] Penetration testing
- [ ] Disaster recovery drill
- [ ] Update documentation
- [ ] Review user access

---

## ⚠️ Common Errors

### "401 Unauthorized"
**Cause:** Invalid or missing API key
```bash
# Fix: Verify key in Authorization header
curl -H "Authorization: Bearer YOUR_EXACT_KEY" ...
```

### "429 Rate Limited"
**Cause:** Exceeded rate limit
```bash
# Wait time is in Retry-After header
# Implement exponential backoff retry
```

### "503 Service Unavailable"
**Cause:** Provider offline or gateway under maintenance
```bash
# Solution: Use failover route, check provider status
```

### "CORS Error"
**Cause:** Frontend domain not whitelisted
```bash
# Fix: Contact admin to whitelist domain
```

---

## 📞 Emergency Contacts

| Issue | Email | Response |
|-------|-------|----------|
| Technical Problem | pv@realtimedetect.com | 2 hours |
| Billing Question | pv@realtimedetect.com | 24 hours |
| Security Issue | security@llm-gateway.com | 1 hour |
| Account Issue | admin@llm-gateway.com | 24 hours |

---

## 🎯 Performance Tips

**Reduce Costs:**
- Use GPT-3.5 instead of GPT-4 when possible
- Cache common responses
- Batch requests when possible
- Monitor and optimize prompts

**Improve Speed:**
- Use streaming for long responses
- Implement connection pooling
- Use regional endpoints
- Enable compression

**Better Reliability:**
- Set up failover routes
- Implement retry logic
- Monitor provider health
- Set request timeouts

---

## 📚 Key Resources

- **Full Manual:** LLM_Gateway_User_Manual.html
- **API Docs:** API_Reference.md
- **PDF Guide:** PDF_GENERATION_GUIDE.md
- **Admin Guide:** See Administration section in full manual

---

**Last Updated:** March 2026 | **Version:** 1.0.0
