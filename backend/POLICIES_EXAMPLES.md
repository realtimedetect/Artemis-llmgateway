# LLM Gateway Policy System - Ready-to-Use Examples

Copy and paste these policies into your gateway to get started immediately!

---

## 🔒 Security Policies

### Block Password References

**Blocks requests asking about passwords**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block Password References",
    "description": "Prevent requests containing password references",
    "scope": "global",
    "pattern": "(?i)\\b(password|passwd|pwd|pass\\s+phrase)\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "Blocks requests asking about or discussing passwords"
  }'
```

### Block API Keys

**Prevents exposure of API credentials**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block API Keys",
    "description": "Prevent requests containing API key patterns",
    "scope": "global",
    "pattern": "(api[_-]?key|api[_-]?secret|token|bearer|x-api-key)['\''\"']?\\s*[=:]",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "Blocks requests with API key patterns"
  }'
```

### Block SSN (Social Security Numbers)

**Prevents exposure of US SSNs**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block SSN",
    "description": "Prevent requests containing social security numbers",
    "scope": "global",
    "pattern": "\\b\\d{3}-\\d{2}-\\d{4}\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "Blocks SSN pattern XXX-XX-XXXX"
  }'
```

### Block Credit Card Numbers

**Prevents exposure of credit card data**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block Credit Cards",
    "description": "Prevent requests containing credit card numbers",
    "scope": "global",
    "pattern": "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{3,4}\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "Blocks credit card patterns"
  }'
```

### Block Email Addresses

**Prevents spam/privacy issues by blocking email collection**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block Email Extraction",
    "description": "Prevent requests trying to extract email addresses",
    "scope": "global",
    "pattern": "(?i)(extract|find|list|get).{0,20}email",
    "target": "content",
    "action": "deny",
    "priority": 50,
    "enabled": false,
    "notes": "Optional: Enable if you want to prevent email extraction requests"
  }'
```

### Block Harmful Instructions

**Blocks requests for dangerous content**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block Harmful Instructions",
    "description": "Block requests for dangerous/harmful content",
    "scope": "global",
    "pattern": "(?i)\\b(bomb|exploit|hack|malware|ransomware|ddos|crack|bypass|jailbreak|injection)\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "Blocks requests for dangerous content"
  }'
```

---

## 📋 Compliance Policies

### GDPR - Block Sensitive Data

**Prevents processing GDPR-sensitive information**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Compliance - GDPR Sensitive Data",
    "description": "Block requests containing GDPR-sensitive information",
    "scope": "global",
    "pattern": "(?i)\\b(passport|license|birth certificate|dob|date of birth|medical|health record|genetic|biometric)\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "GDPR compliance - blocks sensitive personal data"
  }'
```

### HIPAA - Block Health Info

**Prevents processing of health-related information**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Compliance - Block Health Information",
    "description": "Block requests containing health/medical information",
    "scope": "global",
    "pattern": "(?i)\\b(diagnosis|treatment|medication|prescription|patient|hospital|disease|illness|medical record)\\b",
    "target": "content",
    "action": "deny",
    "priority": 100,
    "enabled": false,
    "notes": "Optional: Enable if handling health data - HIPAA compliance"
  }'
```

### PCI-DSS - Block Payment Card Data

**Ensures PCI-DSS compliance by blocking card data requests**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Compliance - No Payment Card Data",
    "description": "Block requests containing payment card information",
    "scope": "global",
    "pattern": "(?i)\\b(credit card|debit card|card number|cvv|expir|card holder|cardholder|expiration)\\b",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true,
    "notes": "PCI-DSS compliance"
  }'
```

---

## 🎯 Model Usage Policies

### Only Production Models

**Allows only approved production models**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Usage - Production Models Only",
    "description": "Only allow production-approved models",
    "scope": "global",
    "pattern": "^(gpt-4|gpt-4-turbo|claude-3-opus|claude-3-sonnet).*",
    "target": "model",
    "action": "allow",
    "priority": 10,
    "enabled": true,
    "notes": "Enforce production model usage"
  }'
```

### No Experimental Models

**Blocks usage of experimental/beta models**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Usage - Block Experimental",
    "description": "Block experimental/beta/alpha models",
    "scope": "global",
    "pattern": "(?i)(experimental|beta|alpha|preview|test|draft|-dev)",
    "target": "model",
    "action": "deny",
    "priority": 10,
    "enabled": true,
    "notes": "Prevents testing with unstable models"
  }'
```

### No Cheap Models (Cost Control)

**Blocks low-cost/low-quality models to maintain quality standards**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Usage - No Cheap Models",
    "description": "Use only quality models, not cheap options",
    "scope": "global",
    "pattern": "(?i)(gpt-3\\.5|davinci|tinydavinci|small)",
    "target": "model",
    "action": "deny",
    "priority": 50,
    "enabled": false,
    "notes": "Optional: Enable if you want to enforce quality models"
  }'
```

---

## 💻 Per-Model Policies

### GPT-4 - Code Generation Only

**Restrict GPT-4 to code-related tasks**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GPT-4 - Code Generation Only",
    "description": "GPT-4 should only be used for code tasks",
    "scope": "local",
    "model_name": "gpt-4",
    "pattern": "(?i)\\b(function|class|def|code|algorithm|library|framework|debug|refactor|test)\\b",
    "target": "content",
    "action": "allow",
    "priority": 100,
    "enabled": false,
    "notes": "Optional: Enable to restrict GPT-4 to coding"
  }'
```

### GPT-3.5 - Block Long Prompts

**Restrict GPT-3.5 to shorter, simpler requests**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GPT-3.5 - Block Long Prompts",
    "description": "Block prompts longer than 2000 chars for cost control",
    "scope": "local",
    "model_name": "gpt-3.5-turbo",
    "pattern": "^.{2000,}$",
    "target": "content_full",
    "action": "deny",
    "priority": 500,
    "enabled": false,
    "notes": "Optional: Enable to limit GPT-3.5 prompt length"
  }'
```

### Claude-3 - Research Only

**Restrict Claude to research/analysis tasks**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Claude-3 - Research Only",
    "description": "Claude should only be used for research/analysis",
    "scope": "local",
    "model_name": "claude-3-opus",
    "pattern": "(?i)\\b(research|analyze|analyze|study|investigate|summarize|extract|find|search)\\b",
    "target": "content",
    "action": "allow",
    "priority": 100,
    "enabled": false,
    "notes": "Optional: Enable to restrict Claude to research"
  }'
```

---

## 👥 User-Based Policies

### Block Development User

**Prevent specific users from production access**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User - Block Dev User",
    "description": "Block development user from production",
    "scope": "global",
    "pattern": "dev-user-id-123",
    "target": "user",
    "action": "deny",
    "priority": 10,
    "enabled": false,
    "notes": "Replace with actual dev user ID"
  }'
```

### Block Suspended User

**Prevent suspended users from access**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User - Block Suspended",
    "description": "Block suspended users",
    "scope": "global",
    "pattern": "(suspended|blacklisted|blocked)",
    "target": "user",
    "action": "deny",
    "priority": 1,
    "enabled": false,
    "notes": "Optional: Enable to block specific user patterns"
  }'
```

---

## 🌐 Provider-Based Policies

### Production - OpenAI Only

**Only allow OpenAI provider**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Provider - OpenAI Only",
    "description": "Production uses OpenAI provider only",
    "scope": "global",
    "pattern": "^openai$",
    "target": "provider",
    "action": "allow",
    "priority": 5,
    "enabled": false,
    "notes": "Optional: Enable to restrict to OpenAI"
  }'
```

### No Azure Provider

**Block Azure OpenAI provider**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Provider - Block Azure",
    "description": "Block Azure OpenAI provider",
    "scope": "global",
    "pattern": "(?i)azure",
    "target": "provider",
    "action": "deny",
    "priority": 50,
    "enabled": false,
    "notes": "Optional: Enable to avoid Azure"
  }'
```

---

## 📊 Content Quality Policies

### Require Specific Format

**Ensure requests are asking for specific format**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Content - Require Format Spec",
    "description": "Require requests to specify output format",
    "scope": "global",
    "pattern": "(json|xml|csv|markdown|plain text|table)",
    "target": "content",
    "action": "allow",
    "priority": 200,
    "enabled": false,
    "notes": "Optional: Require format specification"
  }'
```

### Block Inappropriate Content

**Block NSFW requests**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Content - Block NSFW",
    "description": "Block NSFW/adult content requests",
    "scope": "global",
    "pattern": "(?i)(nsfw|adult|porn|xxx|explicit|sexual|nude)",
    "target": "content",
    "action": "deny",
    "priority": 50,
    "enabled": false,
    "notes": "Optional: Enable for family-friendly environment"
  }'
```

---

## 💰 Cost Control Policies

### Block Very Long Prompts

**Prevent expensive long prompts**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Cost - Block Huge Prompts",
    "description": "Block prompts over 5000 characters",
    "scope": "global",
    "pattern": "^.{5000,}$",
    "target": "content_full",
    "action": "deny",
    "priority": 500,
    "enabled": false,
    "notes": "Cost control: prevents very expensive requests"
  }'
```

### Limit Image Prompts

**Block expensive image analysis requests**

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Cost - Block Image Requests",
    "description": "Block expensive image analysis",
    "scope": "global",
    "pattern": "(?i)(image|photo|picture|screenshot|diagram|chart)",
    "target": "content",
    "action": "deny",
    "priority": 500,
    "enabled": false,
    "notes": "Optional: Enable to prevent image analysis costs"
  }'
```

---

## 🛠️ Quick Copy-Paste Template

Use this template to create your own policies:

```bash
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "YOUR_POLICY_NAME",
    "description": "Description of what this policy does",
    "scope": "global",
    "pattern": "YOUR_REGEX_PATTERN",
    "target": "content",
    "action": "deny",
    "priority": 100,
    "enabled": true,
    "notes": "Optional notes about this policy"
  }'
```

### Variables to Replace:
- `YOUR_POLICY_NAME` - Descriptive name
- `Description of what this policy does` - What it blocks/allows
- `YOUR_REGEX_PATTERN` - Regex pattern to match
- `content` - Field to match (model, content, user, provider, prompt, content_full)
- `deny` - Action (allow or deny)
- `100` - Priority (lower = higher priority)

---

## 🧪 Test Any Policy

Before enabling, test with:

```bash
curl -X POST https://gateway/api/policies/evaluate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "gpt-4",
    "content_text": "Your test prompt here"
  }'
```

---

## 📝 Notes

- Start with disabled policies, test them, then enable
- Use https://regex101.com for pattern testing
- Lower priority numbers are evaluated first
- Deny rules stop evaluation (first deny wins)
- Monitor `/api/policies/{id}/metrics` for effectiveness

---

**Happy Policy Building!** 🎉
