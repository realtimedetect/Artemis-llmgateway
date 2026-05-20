# LLM Gateway Policy System - Quick Start Guide

## 5-Minute Setup

### 1. Create Your First Policy (Block Passwords)

```bash
curl -X POST https://gateway.example.com/api/policies \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Block Password References",
    "description": "Prevent users from asking about passwords",
    "scope": "global",
    "pattern": "(?i)(password|passwd|pwd|secret)",
    "target": "content",
    "action": "deny",
    "priority": 10,
    "enabled": true
  }'
```

### 2. Test the Policy

```bash
curl -X POST https://gateway.example.com/api/policies/evaluate \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "gpt-4",
    "content_text": "What is the password for the database?"
  }'

# Response: allowed = false (blocked!)
```

### 3. Make a Request (Will be Blocked)

```bash
curl -X POST https://gateway.example.com/api/chat/completions \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "What is the password?"}]
  }'

# Response: 403 Forbidden - "Policy 'Block Password References' denied the request"
```

---

## Common Policy Patterns

### Security & Compliance

#### Block PII (Personally Identifiable Information)
```json
{
  "name": "Block PII - SSN",
  "scope": "global",
  "pattern": "\\d{3}-\\d{2}-\\d{4}",
  "target": "content",
  "action": "deny",
  "priority": 5
}
```

#### Block Credit Card Numbers
```json
{
  "name": "Block Credit Card",
  "scope": "global",
  "pattern": "\\d{4}[-\\s]?\\d{4}[-\\s]?\\d{4}[-\\s]?\\d{3,4}",
  "target": "content",
  "action": "deny",
  "priority": 5
}
```

#### Block API Keys
```json
{
  "name": "Block API Keys",
  "scope": "global",
  "pattern": "(api[-_]?key|api[-_]?secret|token|bearer)['\"]?\\s*[:=]",
  "target": "content",
  "action": "deny",
  "priority": 10
}
```

#### GDPR Compliance - Block Sensitive Data
```json
{
  "name": "GDPR - Block Sensitive Data",
  "scope": "global",
  "pattern": "(passport|license|medical|health record|birth certificate)",
  "target": "content",
  "action": "deny",
  "priority": 5
}
```

### Model-Specific Usage

#### Only Allow Specific Models
```json
{
  "name": "Production - Only GPT-4 and Claude",
  "scope": "global",
  "pattern": "^(gpt-4|gpt-4-turbo|claude-3).*",
  "target": "model",
  "action": "allow",
  "priority": 1
}
```

#### Deny Experimental Models
```json
{
  "name": "Block Experimental Models",
  "scope": "global",
  "pattern": "(experimental|beta|alpha|preview|test)",
  "target": "model",
  "action": "deny",
  "priority": 50
}
```

#### GPT-4 Code Generation Only
```json
{
  "name": "GPT-4 - Code Generation",
  "scope": "local",
  "model_name": "gpt-4",
  "pattern": "(function|class|def |import |write.*code|generate.*code|algorithm)",
  "target": "content",
  "action": "allow",
  "priority": 100
}
```

#### Restrict GPT-3.5 to Short Prompts
```json
{
  "name": "GPT-3.5 - Block Long Prompts",
  "scope": "local",
  "model_name": "gpt-3.5-turbo",
  "pattern": "^.{2000,}$",
  "target": "content_full",
  "action": "deny",
  "priority": 500
}
```

### Content Filtering

#### Block Harmful Content
```json
{
  "name": "Block Harmful Instructions",
  "scope": "global",
  "pattern": "(bomb|exploit|hack|malware|ransomware|ddos)",
  "target": "content",
  "action": "deny",
  "priority": 5
}
```

#### Block NSFW Content
```json
{
  "name": "Block NSFW",
  "scope": "global",
  "pattern": "(?i)(explicit|adult|nsfw|porn|xxx)",
  "target": "content",
  "action": "deny",
  "priority": 5
}
```

#### Require Technical Context
```json
{
  "name": "Only Technical Queries",
  "scope": "local",
  "model_name": "code-llama",
  "pattern": "(code|function|algorithm|library|framework|database)",
  "target": "content",
  "action": "allow",
  "priority": 50
}
```

### User & Provider Restrictions

#### Block Specific User
```json
{
  "name": "Block Restricted User",
  "scope": "global",
  "pattern": "(suspended-user-123|blacklisted-user)",
  "target": "user",
  "action": "deny",
  "priority": 1
}
```

#### Only Allow OpenAI Provider
```json
{
  "name": "Production - OpenAI Only",
  "scope": "global",
  "pattern": "^openai$",
  "target": "provider",
  "action": "allow",
  "priority": 1
}
```

---

## API Reference

### List All Policies
```bash
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies

# Filter by scope
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies?scope=global

# Filter by model
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies?model=gpt-4
```

### Get Specific Policy
```bash
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies/{policy_id}
```

### Update Policy
```bash
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}' \
  https://gateway/api/policies/{policy_id}
```

### Delete Policy
```bash
curl -X DELETE -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies/{policy_id}
```

### Test Policy Against Sample Data
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "gpt-4",
    "content_text": "Your test prompt here",
    "prompt_text": "Optional system prompt"
  }' \
  https://gateway/api/policies/evaluate
```

### Get Policies for a Specific Model
```bash
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies/model/gpt-4
```

### Get Policy Metrics (Last 7 Days)
```bash
curl -H "Authorization: Bearer TOKEN" \
  https://gateway/api/policies/{policy_id}/metrics?days=7
```

---

## Priority Strategy

Use these priority ranges for consistent organization:

```
Priority Range    | Purpose
===================================
1 - 10           | CRITICAL (blocking)
                 | - Block PII, passwords
                 | - Security rules
                 |
11 - 50          | HIGH (enforcement)
                 | - Model restrictions
                 | - Compliance
                 |
51 - 200         | MEDIUM (guidelines)
                 | - Usage limits
                 | - Content types
                 |
201 - 500        | LOW (suggestions)
                 | - Warnings
                 | - Best practices
                 |
501+             | CUSTOM (user-specific)
                 | - Edge cases
```

---

## Regex Quick Reference

```regex
Pattern              | Matches
=============================================
hello                | exactly "hello"
(?i)hello            | "hello", "Hello", "HELLO"
(cat|dog)            | "cat" or "dog"
\d{3}-\d{4}          | "123-4567"
\d{3}-\d{2}-\d{4}    | "123-45-6789" (SSN)
[a-z]+               | "abc", "xyz" (letters)
\w+                  | "word123" (alphanumeric)
.{5,}                | any string 5+ chars
^admin               | starts with "admin"
password$            | ends with "password"
.*api.*secret.*      | contains "api" and "secret"
[@#$%]               | any of these chars
[^0-9]               | NOT a digit
\. \( \) \? \* \+   | literal chars (escaped)
```

---

## Escape Special Characters

In regex patterns, escape these characters with backslash (`\`):

```
. ^ $ * + ? { } [ ] \ | ( )
```

Examples:
```
Email pattern: [a-z]+@[a-z]+\.[a-z]+
URL pattern:   https?://\S+
IP address:    \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}
```

---

## Debugging Tips

### Test Pattern Matches
Use https://regex101.com to test patterns:
1. Enter your regex in the pattern field
2. Enter test strings in the test string field
3. Verify matches are correct

### Enable Debug Logging
In Go, log policy evaluations:
```go
log.Printf("Policy %s matched: %v", policy.ID, matches)
```

### Common Mistakes

❌ **Mistake:** Using `.* ` at the start/end unnecessarily
```regex
.*password.*  // Inefficient
```

✅ **Better:** Be specific
```regex
password|passwd|pwd  // More efficient
```

❌ **Mistake:** Forgetting to escape special chars
```regex
file.txt  // Matches "filXtxt" (. = any char)
```

✅ **Better:** Escape the dot
```regex
file\.txt  // Matches only "file.txt"
```

---

## Policy Decision Flow

```
User Request with model="gpt-4", content="my password is 123"
       ↓
Load User's Policies (sorted by priority)
       ↓
Global Policies:
  1. Priority 5: Block PII (SSN pattern) → no match ✓
  2. Priority 10: Block Passwords → MATCH! action=deny → DENY
       ↓
RESULT: 403 Forbidden
       ↓
Log: "Policy 'Block Passwords' denied the request"
```

---

## Best Practices

### ✅ DO

- ✅ Start with broad rules, then get specific
- ✅ Test patterns before deploying
- ✅ Use meaningful names and descriptions
- ✅ Document the business reason for each policy
- ✅ Monitor denied requests for false positives
- ✅ Review and update policies quarterly
- ✅ Use version control for policy definitions

### ❌ DON'T

- ❌ Use overly complex regex patterns
- ❌ Rely on exact string matching when patterns are better
- ❌ Set very low priority for all rules (use proper ordering)
- ❌ Deploy policies without testing
- ❌ Forget to enable policies after creation
- ❌ Use deny-all patterns without proper scoping

---

## Troubleshooting

### Policy Not Blocking Requests?

1. **Is it enabled?**
   ```bash
   curl https://gateway/api/policies/{id} | grep '"enabled":'
   ```

2. **Pattern correct?** Test it:
   ```bash
   curl -X POST https://gateway/api/policies/evaluate \
     -d '{"model_name": "gpt-4", "content_text": "test"}'
   ```

3. **Scope correct?**
   - For global: Leave `model_name` empty
   - For local: Specify exact model name

4. **Priority correct?**
   - Lower numbers = evaluated first
   - First deny wins

### Regex Matching Not Working?

- Use https://regex101.com to test pattern
- Check if pattern needs case-insensitive flag: `(?i)`
- Verify special characters are escaped: `\.` not `.`
- Use raw test strings (copy from actual requests)

### Performance Issues?

- Reduce policy count by consolidating patterns
- Avoid lookahead/lookbehind patterns (complex)
- Put high-priority deny rules first
- Monitor via `/api/policies/{id}/metrics`

---

## Example: Complete Setup

### Step 1: Create Core Security Policies

```bash
# Block passwords
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block Passwords",
    "scope": "global",
    "pattern": "(?i)(password|passwd|pwd)",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true
  }'

# Block API keys
curl -X POST https://gateway/api/policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security - Block API Keys",
    "scope": "global",
    "pattern": "(api.?key|api.?secret|bearer.token)",
    "target": "content",
    "action": "deny",
    "priority": 5,
    "enabled": true
  }'
```

### Step 2: Test Policies

```bash
# Test 1 - Should be allowed
curl -X POST https://gateway/api/policies/evaluate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "gpt-4",
    "content_text": "Write a hello world program"
  }'
# Result: allowed = true ✓

# Test 2 - Should be blocked
curl -X POST https://gateway/api/policies/evaluate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "gpt-4",
    "content_text": "What is my password?"
  }'
# Result: allowed = false ✓
```

### Step 3: Verify in Production

```bash
# This should be blocked
curl -X POST https://gateway/api/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "password"}]
  }'
# Result: 403 Forbidden ✓

# This should work
curl -X POST https://gateway/api/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello world"}]
  }'
# Result: 200 OK ✓
```

---

## Need Help?

- Check pattern with regex tester: https://regex101.com
- Review policy evaluation: POST `/api/policies/evaluate`
- Check policy status: GET `/api/policies`
- View policy metrics: GET `/api/policies/{id}/metrics`
- Check gateway logs for evaluation errors

---

**Last Updated:** April 3, 2026  
**Version:** 1.0
