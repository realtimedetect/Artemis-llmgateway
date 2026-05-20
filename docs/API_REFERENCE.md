# LLM Gateway - Complete API Reference

**API Version:** 1.0.0  
**Base URL:** `https://api.llm-gateway.com`  
**Authentication:** Bearer Token (JWT)

---

## Authentication

All API requests require authentication via Bearer token in the `Authorization` header:

```bash
Authorization: Bearer YOUR_API_KEY
```

### Obtaining an API Key

1. Log in to LLM Gateway dashboard
2. Navigate to **API Keys** section
3. Click **+ Generate New Key**
4. Copy the secret key (displayed only once)
5. Use in `Authorization: Bearer {secret_key}` header

---

## Response Format

### Success Response (2xx)
```json
{
  "success": true,
  "data": { /* response data */ },
  "timestamp": "2024-03-15T10:30:00Z"
}
```

### Error Response (4xx, 5xx)
```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": { /* additional context */ }
  },
  "timestamp": "2024-03-15T10:30:00Z"
}
```

---

## Rate Limiting

Rate limits are enforced per API key. Exceeded limits return `429 Too Many Requests`.

Response headers include:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1699456800
Retry-After: 30
```

---

## Core Endpoints

---

## 1. Chat Completions (OpenAI Compatible)

### POST `/v1/chat/completions`

Send a chat message to an LLM model.

**Request:**
```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is AI?"}
  ],
  "temperature": 0.7,
  "max_tokens": 150,
  "top_p": 1.0,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "stream": false
}
```

**Response:**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1699456789,
  "model": "gpt-4",
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 8,
    "total_tokens": 33
  },
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Artificial Intelligence (AI) is..."
      },
      "finish_reason": "stop",
      "index": 0
    }
  ]
}
```

**Query Parameters:**
- `model` (required): Model name (gpt-4, gpt-3.5-turbo, claude-2, etc.)
- `messages` (required): Array of message objects
- `temperature` (optional, 0-2): Randomness control (default: 0.7)
- `max_tokens` (optional): Max response length
- `top_p` (optional, 0-1): Nucleus sampling
- `stream` (optional): Enable response streaming

**Error Codes:**
- `400` - Invalid request format
- `401` - Unauthorized (missing/invalid key)
- `429` - Rate limited
- `503` - Provider unavailable

---

## 2. Embeddings

### POST `/v1/embeddings`

Generate embeddings for text input.

**Request:**
```json
{
  "model": "text-embedding-3-small",
  "input": "The quick brown fox jumps over the lazy dog"
}
```

**Response:**
```json
{
  "object": "list",
  "model": "text-embedding-3-small",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.123, -0.456, 0.789, ...]
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "total_tokens": 15
  }
}
```

---

## 3. Authentication

### POST `/api/auth/login`

Authenticate and receive JWT token.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "secure_password"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "user-123",
      "email": "user@example.com",
      "name": "John Doe",
      "role": "developer"
    },
    "expiresIn": 86400
  }
}
```

---

## 4. API Keys Management

### POST `/api/keys`

Create a new API key.

**Request:**
```json
{
  "name": "Production API Key",
  "team_id": "team-123",
  "rate_limit": 1000,
  "expires_at": "2024-12-31T23:59:59Z"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "key-abc123",
    "secret": "sk_prod_abc123def456...",
    "name": "Production API Key",
    "team_id": "team-123",
    "created_at": "2024-03-15T10:30:00Z",
    "expires_at": "2024-12-31T23:59:59Z",
    "rate_limit": 1000,
    "status": "active"
  }
}
```

⚠️ **Note:** Secret is shown only once. Store securely.

### GET `/api/keys`

List all API keys for the authenticated user.

**Query Parameters:**
- `limit` (optional, default: 50): Max results
- `offset` (optional, default: 0): Pagination offset
- `team_id` (optional): Filter by team

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "key-abc123",
      "name": "Production API Key",
      "team_id": "team-123",
      "created_at": "2024-03-15T10:30:00Z",
      "last_used": "2024-03-20T14:20:00Z",
      "status": "active",
      "rate_limit": 1000
    }
  ],
  "pagination": {
    "total": 5,
    "limit": 50,
    "offset": 0
  }
}
```

### GET `/api/keys/{key-id}`

Get details for a specific API key.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "key-abc123",
    "name": "Production API Key",
    "team_id": "team-123",
    "created_at": "2024-03-15T10:30:00Z",
    "expires_at": "2024-12-31T23:59:59Z",
    "last_used": "2024-03-20T14:20:00Z",
    "request_count": 15234,
    "status": "active",
    "rate_limit": 1000
  }
}
```

### PUT `/api/keys/{key-id}`

Update an API key.

**Request:**
```json
{
  "name": "Updated Key Name",
  "rate_limit": 2000,
  "expires_at": "2025-12-31T23:59:59Z"
}
```

### POST `/api/keys/{key-id}/rotate`

Rotate (regenerate) an API key secret.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "key-abc123",
    "secret": "sk_prod_new_secret_xyz789...",
    "rotated_at": "2024-03-15T10:30:00Z"
  }
}
```

⚠️ **Note:** Old secret becomes invalid immediately.

### DELETE `/api/keys/{key-id}`

Delete (revoke) an API key.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "key-abc123",
    "deleted_at": "2024-03-15T10:30:00Z"
  }
}
```

---

## 5. Providers

### GET `/api/providers`

List all connected LLM providers.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "provider-openai",
      "name": "OpenAI",
      "type": "openai",
      "status": "online",
      "models": ["gpt-4", "gpt-3.5-turbo"],
      "latency_ms": 145,
      "error_rate": 0.02,
      "last_check": "2024-03-15T10:28:00Z"
    },
    {
      "id": "provider-azure",
      "name": "Azure OpenAI",
      "type": "azure",
      "status": "online",
      "models": ["gpt-4", "gpt-3.5-turbo"],
      "latency_ms": 180,
      "error_rate": 0.01,
      "last_check": "2024-03-15T10:29:00Z"
    }
  ]
}
```

### POST `/api/providers`

Create a new provider connection.

**Request:**
```json
{
  "name": "OpenAI Production",
  "type": "openai",
  "api_key": "sk-...",
  "organization_id": "org-...",
  "enabled": true,
  "priority": 1
}
```

### PUT `/api/providers/{provider-id}`

Update provider configuration.

**Request:**
```json
{
  "priority": 2,
  "enabled": false,
  "rate_limit": 500
}
```

### GET `/api/providers/{provider-id}/health`

Get real-time health status of a provider.

**Response:**
```json
{
  "success": true,
  "data": {
    "provider_id": "provider-openai",
    "status": "online",
    "latency_ms": 152,
    "error_rate": 0.02,
    "quota_remaining": 9850,
    "quota_reset": "2024-03-16T00:00:00Z",
    "last_error": null,
    "last_check": "2024-03-15T10:30:00Z"
  }
}
```

---

## 6. Routes

### GET `/api/routes`

List all configured routes.

**Query Parameters:**
- `model` (optional): Filter by model
- `limit` (optional): Max results

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "route-prod-gpt4",
      "name": "Production GPT-4",
      "description": "Primary route for GPT-4 requests",
      "model_patterns": ["gpt-4"],
      "providers": [
        {
          "provider_id": "provider-openai",
          "priority": 1,
          "weight": 100
        },
        {
          "provider_id": "provider-azure",
          "priority": 2,
          "weight": 0
        }
      ],
      "retry_policy": {
        "max_retries": 3,
        "backoff_ms": 100
      },
      "enabled": true
    }
  ]
}
```

### POST `/api/routes`

Create a new route.

**Request:**
```json
{
  "name": "Production GPT-4",
  "description": "Primary route for GPT-4",
  "model_patterns": ["gpt-4"],
  "providers": [
    {
      "provider_id": "provider-openai",
      "priority": 1,
      "weight": 100
    }
  ],
  "retry_policy": {
    "max_retries": 3,
    "backoff_ms": 100
  }
}
```

### PUT `/api/routes/{route-id}`

Update a route configuration.

### DELETE `/api/routes/{route-id}`

Delete a route.

---

## 7. User Groups/Teams

### POST `/api/user-groups`

Create a new user group.

**Request:**
```json
{
  "name": "Data Science Team",
  "description": "Team working on ML models"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "group-ds-team",
    "name": "Data Science Team",
    "description": "Team working on ML models",
    "owner_id": "user-123",
    "member_count": 0,
    "created_at": "2024-03-15T10:30:00Z"
  }
}
```

### GET `/api/user-groups`

List all groups (paginated).

**Query Parameters:**
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

### GET `/api/user-groups/{group-id}`

Get group details.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "group-ds-team",
    "name": "Data Science Team",
    "description": "Team working on ML models",
    "owner_id": "user-123",
    "member_count": 5,
    "created_at": "2024-03-15T10:30:00Z",
    "members": [
      {
        "user_id": "user-456",
        "email": "john@example.com",
        "role": "member",
        "joined_at": "2024-03-15T10:35:00Z"
      }
    ]
  }
}
```

### POST `/api/user-groups/{group-id}/members`

Add a member to a group.

**Request:**
```json
{
  "email": "newmember@example.com",
  "role": "member"
}
```

### GET `/api/user-groups/{group-id}/analytics`

Get group usage analytics.

**Query Parameters:**
- `start_date` (optional): ISO 8601 date
- `end_date` (optional): ISO 8601 date

**Response:**
```json
{
  "success": true,
  "data": {
    "group_id": "group-ds-team",
    "period": "2024-03-01 to 2024-03-15",
    "total_requests": 5230,
    "total_tokens": 1250000,
    "total_cost": 45.75,
    "average_latency_ms": 156,
    "error_rate": 0.02,
    "active_members": 4,
    "top_models": [
      {"model": "gpt-4", "requests": 3100, "cost": 30.00},
      {"model": "gpt-3.5-turbo", "requests": 2130, "cost": 15.75}
    ]
  }
}
```

### GET `/api/user-groups/{group-id}/breakdown`

Get per-member usage breakdown.

**Response:**
```json
{
  "success": true,
  "data": {
    "group_id": "group-ds-team",
    "members": [
      {
        "user_id": "user-456",
        "email": "john@example.com",
        "requests": 1500,
        "tokens": 350000,
        "cost": 12.50
      },
      {
        "user_id": "user-789",
        "email": "jane@example.com",
        "requests": 3730,
        "tokens": 900000,
        "cost": 33.25
      }
    ]
  }
}
```

---

## 8. Usage Analytics

### GET `/api/analytics/usage`

Get overall usage statistics.

**Query Parameters:**
- `start_date` (optional)
- `end_date` (optional)
- `granularity` (optional): hour, day, week, month

**Response:**
```json
{
  "success": true,
  "data": {
    "total_requests": 50230,
    "total_tokens": 12500000,
    "total_cost": 457.50,
    "average_latency_ms": 165,
    "error_rate": 0.019,
    "active_users": 12,
    "models": {
      "gpt-4": 31000,
      "gpt-3.5-turbo": 19230
    },
    "providers": {
      "openai": 40000,
      "azure": 10230
    }
  }
}
```

### GET `/api/analytics/requests`

Get detailed request history.

**Query Parameters:**
- `limit` (optional, default: 100)
- `offset` (optional, default: 0)
- `model` (optional): Filter by model
- `status` (optional): success, error, timeout
- `start_date` (optional)
- `end_date` (optional)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "request_id": "req-abc123",
      "timestamp": "2024-03-15T10:30:00Z",
      "user_id": "user-123",
      "api_key_id": "key-abc123",
      "model": "gpt-4",
      "provider": "openai",
      "status": "success",
      "latency_ms": 156,
      "input_tokens": 25,
      "output_tokens": 8,
      "total_tokens": 33,
      "cost": 0.001
    }
  ],
  "pagination": {
    "total": 50230,
    "limit": 100,
    "offset": 0
  }
}
```

### GET `/api/analytics/cost-breakdown`

Get cost analytics by various dimensions.

**Query Parameters:**
- `start_date` (required): ISO 8601 date
- `end_date` (required): ISO 8601 date
- `group_by` (optional): model, provider, team, user (default: provider)

**Response:**
```json
{
  "success": true,
  "data": {
    "period": "2024-03-01 to 2024-03-31",
    "total_cost": 1234.56,
    "by_provider": {
      "openai": 800.00,
      "azure": 300.00,
      "anthropic": 134.56
    },
    "by_model": {
      "gpt-4": 600.00,
      "gpt-3.5-turbo": 400.00,
      "claude-2": 234.56
    },
    "by_team": {
      "data-science": 800.00,
      "ml-engineering": 434.56
    }
  }
}
```

---

## 9. Prompts Management

### POST `/api/prompts/templates`

Create a new prompt template.

**Request:**
```json
{
  "name": "Code Review",
  "category": "development",
  "description": "Review code for quality and security",
  "content": "You are a code reviewer. Review the following code and identify...",
  "variables": [
    {
      "name": "code",
      "type": "text",
      "description": "Code to review",
      "required": true
    }
  ]
}
```

### GET `/api/prompts/templates`

List prompt templates.

### GET `/api/prompts/templates/{template-id}/versions`

Get all versions of a template.

### POST `/api/prompts/templates/{template-id}/versions`

Create a new template version.

### PUT `/api/prompts/templates/{template-id}/active`

Set active version.

**Request:**
```json
{
  "version_id": "v2.0"
}
```

### POST `/api/prompts/test`

Test a prompt template.

**Request:**
```json
{
  "template_id": "template-code-review",
  "version_id": "v1.0",
  "variables": {
    "code": "function add(a, b) { return a + b; }"
  },
  "model": "gpt-4"
}
```

---

## 10. Costs Management

### GET `/api/costs`

List cost rules.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "cost-gpt4-input",
      "model": "gpt-4",
      "cost_type": "input",
      "price_per_1k_tokens": 0.03,
      "effective_date": "2024-03-01T00:00:00Z"
    },
    {
      "id": "cost-gpt4-output",
      "model": "gpt-4",
      "cost_type": "output",
      "price_per_1k_tokens": 0.06,
      "effective_date": "2024-03-01T00:00:00Z"
    }
  ]
}
```

### POST `/api/costs`

Create a cost rule.

**Request:**
```json
{
  "model": "gpt-4",
  "cost_type": "input",
  "price_per_1k_tokens": 0.03,
  "effective_date": "2024-03-01T00:00:00Z"
}
```

### PUT `/api/keys/{key-id}/group`

Assign API key to cost group.

**Request:**
```json
{
  "cost_group_id": "group-prod"
}
```

---

## Error Reference

| Code | Status | Meaning |
|------|--------|---------|
| `INVALID_REQUEST` | 400 | Request format/validation error |
| `UNAUTHORIZED` | 401 | Missing/invalid API key |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `RATE_LIMIT_EXCEEDED` | 429 | Rate limit hit |
| `CONFLICT` | 409 | Resource conflict (e.g., duplicate) |
| `INTERNAL_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | Provider/service down |

---

## Webhook Events (Enterprise)

Webhooks alert your systems to important events:

- `request.completed` - API request finished
- `request.failed` - API request failed
- `cost.threshold_exceeded` - Budget limit reached
- `provider.offline` - Provider went offline
- `key.expiring` - API key expiring soon

---

## Rate Limiting Strategy

Implement exponential backoff:

```python
import time
import random

def call_with_backoff(api_func, max_retries=3):
    for attempt in range(max_retries):
        try:
            return api_func()
        except RateLimitError as e:
            if attempt == max_retries - 1:
                raise
            wait_time = (2 ** attempt) + random.uniform(0, 1)
            print(f"Rate limited. Waiting {wait_time:.1f}s...")
            time.sleep(wait_time)
```

---

## SDKs & Libraries

### Official SDKs
- Python: `pip install llm-gateway`
- JavaScript: `npm install @llm-gateway/client`
- Go: `go get github.com/llm-gateway/go-sdk`

### Compatible Libraries
- Works with OpenAI SDKs (just change base URL)
- Compatible with LangChain, LlamaIndex
- REST/HTTP compatible with any language

---

**Last Updated:** March 2026 | **Version:** 1.0.0
