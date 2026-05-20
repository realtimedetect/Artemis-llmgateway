# LLM Gateway - Groups API Reference

## Overview
The User Groups feature allows you to organize users into teams and track token usage and costs by team.

---

## API Endpoints

### Base URL
```
http://localhost:8080/api
```

### Authentication
All endpoints require JWT Bearer token:
```
Authorization: Bearer <jwt_token>
```

---

## Group Management

### 1. Create Group
**POST** `/user-groups`

**Request Body**:
```json
{
  "name": "Data Science Team",
  "description": "Team analyzing ML models"
}
```

**Response** (201):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "owner_id": "user-123",
  "name": "Data Science Team",
  "description": "Team analyzing ML models",
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### 2. List Groups
**GET** `/user-groups`

**Query Parameters**:
- `limit` (optional): Max results, default 50
- `offset` (optional): Pagination offset, default 0

**Response** (200):
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "owner_id": "user-123",
    "name": "Data Science Team",
    "description": "Team analyzing ML models",
    "member_count": 5,
    "created_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### 3. Get Group Details
**GET** `/user-groups/{groupID}`

**Response** (200):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "owner_id": "user-123",
  "name": "Data Science Team",
  "description": "Team analyzing ML models",
  "member_count": 5,
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### 4. Update Group
**PUT** `/user-groups/{groupID}`

**Request Body**:
```json
{
  "name": "Data Science Team",
  "description": "Team analyzing ML models - Updated"
}
```

**Response** (200):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "owner_id": "user-123",
  "name": "Data Science Team",
  "description": "Team analyzing ML models - Updated",
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### 5. Delete Group
**DELETE** `/user-groups/{groupID}`

**Response** (204 No Content)

Note: Only group owner can delete. Deletes all associated memberships.

---

## Member Management

### 6. Add Member to Group
**POST** `/user-groups/{groupID}/members`

**Request Body**:
```json
{
  "email": "analyst@example.com",
  "role": "member"
}
```

**Role Options**:
- `member` - Can view group analytics
- `admin` - Can manage group and members

**Response** (201):
```json
{
  "id": "member-456",
  "group_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user-456",
  "email": "analyst@example.com",
  "role": "member",
  "joined_at": "2024-01-15T11:00:00Z"
}
```

**Errors**:
- `404 User not found` - Email doesn't exist in system
- `409 Conflict` - User already member of group
- `403 Forbidden` - Only group owner can add members

---

### 7. List Group Members
**GET** `/user-groups/{groupID}/members`

**Query Parameters**:
- `limit` (optional): Max results, default 50
- `offset` (optional): Pagination offset, default 0

**Response** (200):
```json
[
  {
    "id": "member-456",
    "group_id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "user-456",
    "email": "analyst@example.com",
    "role": "member",
    "joined_at": "2024-01-15T11:00:00Z"
  }
]
```

---

### 8. Remove Member from Group
**DELETE** `/user-groups/{groupID}/members/{memberID}`

**Response** (204 No Content)

**Errors**:
- `403 Forbidden` - Only group owner can remove members

---

## Analytics

### 9. Get Group Analytics
**GET** `/user-groups/{groupID}/analytics`

**Query Parameters**:
- `period` (optional): Time period for analytics
  - `today` - Last 24 hours (default)
  - `7d` - Last 7 days
  - `30d` - Last 30 days

**Response** (200):
```json
{
  "group_id": "550e8400-e29b-41d4-a716-446655440000",
  "period": "30d",
  "total_requests": 457,
  "total_tokens": 125000,
  "total_cost_usd": 12.50,
  "avg_latency_ms": 245,
  "top_model": "gpt-4o-mini",
  "top_provider": "openai",
  "member_count": 5,
  "start_date": "2023-12-15",
  "end_date": "2024-01-15"
}
```

**Notes**:
- `total_tokens` is approximate based on request estimates
- `total_cost_usd` calculated from cost rules per provider
- `avg_latency_ms` is average response time
- Includes all group members' requests

---

### 10. Get Member Breakdown
**GET** `/user-groups/{groupID}/breakdown`

**Query Parameters**:
- `period` (optional): Time period for analytics
  - `today` - Last 24 hours (default)
  - `7d` - Last 7 days
  - `30d` - Last 30 days

**Response** (200):
```json
[
  {
    "user_id": "user-456",
    "email": "analyst@example.com",
    "total_requests": 150,
    "total_tokens": 45000,
    "total_cost_usd": 4.50,
    "avg_latency_ms": 230
  },
  {
    "user_id": "user-789",
    "email": "lead@example.com",
    "total_requests": 307,
    "total_tokens": 80000,
    "total_cost_usd": 8.00,
    "avg_latency_ms": 255
  }
]
```

**Sorted By**:
- Default: Total cost (descending)
- Shows per-member breakdown

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request body",
  "details": "name is required"
}
```

### 401 Unauthorized
```json
{
  "error": "Missing or invalid token"
}
```

### 403 Forbidden
```json
{
  "error": "You do not have permission to perform this action"
}
```

### 404 Not Found
```json
{
  "error": "Group not found"
}
```

### 409 Conflict
```json
{
  "error": "User already member of this group"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error",
  "error_id": "req-123456"
}
```

---

## cURL Examples

### Create Group
```bash
curl -X POST http://localhost:8080/api/user-groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Backend Team",
    "description": "Backend developers"
  }'
```

### List Groups
```bash
curl -X GET http://localhost:8080/api/user-groups \
  -H "Authorization: Bearer $TOKEN"
```

### Add Member
```bash
curl -X POST http://localhost:8080/api/user-groups/550e8400-e29b-41d4-a716-446655440000/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "dev@example.com",
    "role": "member"
  }'
```

### Get Analytics (30 days)
```bash
curl -X GET "http://localhost:8080/api/user-groups/550e8400-e29b-41d4-a716-446655440000/analytics?period=30d" \
  -H "Authorization: Bearer $TOKEN"
```

### Get Member Breakdown
```bash
curl -X GET "http://localhost:8080/api/user-groups/550e8400-e29b-41d4-a716-446655440000/breakdown?period=30d" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Database Schema

### user_groups table
```sql
CREATE TABLE user_groups (
  id CHAR(36) PRIMARY KEY,
  owner_id CHAR(36) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (owner_id) REFERENCES users(id),
  UNIQUE KEY unique_owner_name (owner_id, name)
);
```

### user_group_members table
```sql
CREATE TABLE user_group_members (
  id CHAR(36) PRIMARY KEY,
  group_id CHAR(36) NOT NULL,
  user_id CHAR(36) NOT NULL,
  role ENUM('member', 'admin') DEFAULT 'member',
  joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (group_id) REFERENCES user_groups(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE KEY unique_group_member (group_id, user_id)
);
```

### requests table (modified)
```sql
ALTER TABLE requests ADD COLUMN user_group_id CHAR(36) NULL;
ALTER TABLE requests ADD FOREIGN KEY (user_group_id) REFERENCES user_groups(id);
CREATE INDEX idx_requests_user_group_created ON requests(user_group_id, created_at);
```

---

## Permissions Model

| Action | Owner | Admin | Member | Other |
|--------|-------|-------|--------|-------|
| Create Group | ✅ User | ✅ User | ✅ User | ❌ |
| View Group | ✅ | ✅ | ✅ | ❌ |
| Update Group | ✅ Owner | ❌ | ❌ | ❌ |
| Delete Group | ✅ Owner | ❌ | ❌ | ❌ |
| Add Member | ✅ Owner | ✅ | ❌ | ❌ |
| Remove Member | ✅ Owner | ✅ | ❌ | ❌ |
| View Analytics | ✅ | ✅ | ✅ | ❌ |
| View Breakdown | ✅ | ✅ | ✅ | ❌ |

---

## Rate Limiting

All endpoints follow the same rate limiting:
- IP-based: 100 requests per minute per IP
- User-based: 1000 requests per minute per authenticated user

**Rate Limit Headers**:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1705327200
```

---

## Changelog

### v1.0.0 (2024-01-15)
- ✨ Initial release of Groups feature
- 10 new endpoints for group management
- Analytics and member breakdown
- Period-based filtering (today/7d/30d)
