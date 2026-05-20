# LLM Gateway - Quick Testing Checklist

## 🚀 Pre-Testing Setup

### Environment Variables
```bash
# Backend .env
DB_USER=llm_user
DB_PASSWORD=secure_password
DB_HOST=localhost
DB_PORT=3306
DB_NAME=llm_gatway
JWT_SECRET=your-secret-key-min-32-chars!!!!!
DEFAULT_ADMIN_EMAIL=admin@llm-gatway.local
DEFAULT_ADMIN_PASSWORD_BCRYPT=$2a$10$9n4l4PjeSi4OXMlcdrzmi.VfSv1ofqdVH9hN6/3rA3Pt0ECNDVJUe
PORT=8080

# Frontend .env.local
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### Startup Commands
```bash
# Terminal 1: Backend
cd backend && go run cmd/server/main.go

# Terminal 2: Frontend
cd frontend && npm install && npm run dev

# Terminal 3: Database (if needed)
docker-compose up
```

---

## ✅ Phase 1: Application Startup

- [ ] Backend compiles without errors
- [ ] Backend logs: "LLM Gateway server running on :8080"
- [ ] Frontend builds without errors
- [ ] Frontend accessible at `http://localhost:3000`
- [ ] Database connection successful
- [ ] Migrations completed on startup
- [ ] No error messages in console

---

## ✅ Phase 2: Authentication

- [ ] Navigate to `http://localhost:3000/login`
- [ ] Login page loads
- [ ] Enter: `admin@llm-gatway.local` / `admin123`
- [ ] Click "Sign in"
- [ ] Redirect to `/dashboard`
- [ ] JWT token in `localStorage`
- [ ] Sidebar shows logged-in email

---

## ✅ Phase 3: Dashboard Pages (All Load)

Quick check each page loads without 404/500:

- [ ] `/dashboard` - Overview page
- [ ] `/dashboard/chat` - Chat interface
- [ ] `/dashboard/keys` - API keys
- [ ] `/dashboard/providers` - Provider management
- [ ] `/dashboard/routes` - LLM routes
- [ ] `/dashboard/prompts` - Prompt templates
- [ ] `/dashboard/costs` - Cost tracking
- [ ] `/dashboard/groups` - Groups & Teams ⭐
- [ ] `/dashboard/observability` - Metrics
- [ ] `/dashboard/cache` - Cache config
- [ ] `/dashboard/audits` - Audit logs
- [ ] `/dashboard/users` - User management

---

## ✅ Phase 4: Groups Feature (Core)

### Create Group
```
1. Navigate to /dashboard/groups
2. Click "Create New Group"
3. Enter:
   - Name: "Test Group"
   - Description: "Testing"
4. Click "Create"
```

Verify:
- [ ] Group appears in left sidebar
- [ ] "Create New Group" button visible
- [ ] Group detail panel appears when selected

### View Group Details
```
1. Click group in sidebar
```

Verify:
- [ ] Group name displays
- [ ] Members section empty (no members yet)
- [ ] Analytics shows: Requests=0, Tokens=0, Cost=$0

---

## ✅ Phase 5: Member Management

### Create Test Users (First!)
```
1. Navigate to /dashboard/users
2. Click "Create New User"
3. Create 3 test users:
   - user1@example.com (password can be auto-generated)
   - user2@example.com
   - user3@example.com
4. Note the emails
```

Verify:
- [ ] All 3 users appear in user list
- [ ] Users have email column
- [ ] Users exist in database

### Add Members to Group
```
1. Go to /dashboard/groups
2. Select test group
3. Click "Add Member" in right panel
4. Enter: user1@example.com
5. Select role: "member"
6. Click "Add Member"
```

Verify:
- [ ] Member appears in members table
- [ ] Email displays correctly
- [ ] No errors in console
- [ ] Member count increments

### Add More Members
```
1. Repeat for user2@example.com and user3@example.com
```

Verify:
- [ ] All 3 members in table
- [ ] Member count shows 3
- [ ] All emails correct

---

## ✅ Phase 6: Analytics (Before Requests)

### View Initial Analytics
```
1. Group selected with 3 members
2. View analytics section
```

Verify:
- [ ] Total Requests: 0
- [ ] Total Tokens: 0
- [ ] Total Cost: $0.00
- [ ] Avg Latency: 0 ms
- [ ] Top Model: "N/A"
- [ ] Top Provider: "N/A"

### Member Breakdown
```
1. Scroll to "Usage By Member" table
```

Verify:
- [ ] Table shows 3 members
- [ ] All have 0 requests
- [ ] All have 0 cost

---

## ✅ Phase 7: Period Filtering

### Test Period Buttons
```
1. Click "Today"
2. Click "7d"
3. Click "30d"
```

Verify:
- [ ] Each button click refreshes data
- [ ] Analytics recalculate
- [ ] Breakdown refreshes
- [ ] No errors

---

## ✅ Phase 8: Generate Test Data

### Make API Request
```
1. Navigate to /dashboard/chat
2. Select model: "gpt-4o-mini"
3. Type message: "Hello test"
4. Configure API key with valid provider
5. Send message
```

Verify:
- [ ] Message sends
- [ ] Response returns
- [ ] No errors in Network tab

### Return to Groups Analytics
```
1. Go to /dashboard/groups
2. Select same group
3. View analytics
```

Verify:
- [ ] Total Requests: >= 1
- [ ] Total Tokens: > 0
- [ ] Total Cost: > $0.00
- [ ] Avg Latency: > 0 ms
- [ ] Top Model: "gpt-4o-mini"

### Member Breakdown
```
1. View "Usage By Member" table
2. Find member who made request
```

Verify:
- [ ] Member shows 1+ request
- [ ] Member shows tokens used
- [ ] Member shows cost charged
- [ ] Other members show 0

---

## ✅ Phase 9: Multi-User Requests

### Switch Users (Simulate)
```
1. Make requests via /dashboard/chat again
2. Go back to /dashboard/groups
```

Verify:
- [ ] Analytics increment correctly
- [ ] Each member's breakdown updates
- [ ] Total group cost reflects all members

---

## ✅ Phase 10: Member Removal

### Remove a Member
```
1. Go to /dashboard/groups
2. Select group
3. In members table, click member row
4. Click trash icon
5. Confirm deletion
```

Verify:
- [ ] Member removed from table
- [ ] Member count decrements
- [ ] Analytics recalculate (member no longer included)
- [ ] Breakdown excludes removed member

---

## ✅ Phase 11: Group Deletion

### Delete Group
```
1. In groups sidebar
2. Right-click or find delete icon
3. Confirm deletion
```

Verify:
- [ ] Group removed from sidebar
- [ ] Detail panel clears
- [ ] Can create new group
- [ ] No errors

---

## ✅ Phase 12: API Testing (Advanced)

### Test with cURL

#### Create Group
```bash
curl -X POST http://localhost:8080/api/user-groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"API Test","description":"Testing API"}'
```

Verify:
- [ ] Returns 201 Created
- [ ] Response includes group ID
- [ ] Group appears in API list

#### List Groups
```bash
curl -X GET http://localhost:8080/api/user-groups \
  -H "Authorization: Bearer $TOKEN"
```

Verify:
- [ ] Returns 200 OK
- [ ] Array of groups returns
- [ ] Can parse JSON

#### Add Member
```bash
GROUP_ID="..." # from above
curl -X POST http://localhost:8080/api/user-groups/$GROUP_ID/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"user1@example.com","role":"member"}'
```

Verify:
- [ ] Returns 201 Created
- [ ] Member ID included
- [ ] Timestamp accurate

#### Get Analytics
```bash
curl -X GET "http://localhost:8080/api/user-groups/$GROUP_ID/analytics?period=30d" \
  -H "Authorization: Bearer $TOKEN"
```

Verify:
- [ ] Returns 200 OK
- [ ] JSON with metrics
- [ ] total_requests >= 0
- [ ] total_cost_usd >= 0

#### Get Breakdown
```bash
curl -X GET "http://localhost:8080/api/user-groups/$GROUP_ID/breakdown?period=30d" \
  -H "Authorization: Bearer $TOKEN"
```

Verify:
- [ ] Returns 200 OK
- [ ] Array of members
- [ ] Per-member metrics
- [ ] Sorted by cost

---

## ✅ Phase 13: Error Cases

### Test Error Handling

#### Non-existent Group
```bash
curl -X GET http://localhost:8080/api/user-groups/invalid-id \
  -H "Authorization: Bearer $TOKEN"
```

Verify:
- [ ] Returns 404 Not Found
- [ ] Error message clear

#### Missing Token
```bash
curl -X GET http://localhost:8080/api/user-groups
```

Verify:
- [ ] Returns 401 Unauthorized
- [ ] Error message present

#### Add Non-existent User
```bash
curl -X POST http://localhost:8080/api/user-groups/$GROUP_ID/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"nonexistent@example.com","role":"member"}'
```

Verify:
- [ ] Returns 404 Not Found
- [ ] Message: "User not found"

#### Add Duplicate Member
```bash
# Add user1 twice
curl -X POST http://localhost:8080/api/user-groups/$GROUP_ID/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"user1@example.com","role":"member"}'

# Try again
curl -X POST http://localhost:8080/api/user-groups/$GROUP_ID/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"email":"user1@example.com","role":"member"}'
```

Verify:
- [ ] First call: 201 Created
- [ ] Second call: 409 Conflict
- [ ] Error: "already member"

---

## ✅ Phase 14: UI/UX

### Frontend Interactions
- [ ] Buttons are clickable
- [ ] Forms validate input
- [ ] Error messages display
- [ ] Loading states show (if applicable)
- [ ] No layout shifts
- [ ] Responsive on mobile (if mobile testing)
- [ ] Icons render correctly
- [ ] Colors/styling consistent

### Navigation
- [ ] Sidebar links work
- [ ] Breadcrumbs accurate (if present)
- [ ] Back buttons work
- [ ] URLs match routes
- [ ] Page titles update

### Data Display
- [ ] Numbers formatted correctly
- [ ] Tables sortable (if enabled)
- [ ] Pagination works (if > 50 items)
- [ ] Empty states message
- [ ] Loading spinner shows

---

## ✅ Phase 15: Browser Console

### JavaScript Errors
```bash
# Open DevTools: F12
# Console tab
```

Verify:
- [ ] No red error messages
- [ ] No undefined variable warnings
- [ ] No CORS errors
- [ ] Network requests successful (X icons in Network tab)

### Network Requests
```bash
# Network tab
```

Verify:
- [ ] All API calls 200/201 (green)
- [ ] No 404/500 responses (red)
- [ ] Response payloads are JSON
- [ ] Request headers include Authorization
- [ ] Response headers valid

---

## ✅ Phase 16: Database Verification (Optional)

### Check Database

```bash
mysql -u llm_user -p llm_gatway
```

```sql
-- Verify tables exist
SHOW TABLES;

-- Check user_groups
SELECT * FROM user_groups;

-- Check members
SELECT * FROM user_group_members;

-- Check requests with group_id
SELECT COUNT(*) FROM requests WHERE user_group_id IS NOT NULL;

-- Verify indexes
SHOW INDEX FROM requests WHERE Column_name = 'user_group_id';
```

Verify:
- [ ] 3 new tables exist
- [ ] Groups table has data
- [ ] Members table populated
- [ ] Indexes present
- [ ] Foreign keys set

---

## 📋 Summary

**Total Checkpoints**: 150+

**Critical Items** (Must Pass):
- [ ] Login works
- [ ] Create group works
- [ ] Add members works
- [ ] View analytics works
- [ ] All 10 API endpoints accessible
- [ ] No database errors
- [ ] No console errors
- [ ] Period filtering works
- [ ] Member breakdown accurate
- [ ] Error cases handled

---

## 🐛 Issue Tracking

| Issue | Location | Severity | Status |
|-------|----------|----------|--------|
| | | | |
| | | | |
| | | | |

---

## 📊 Test Results

| Category | Pass | Fail | N/A | Notes |
|----------|------|------|-----|-------|
| Startup | | | | |
| Auth | | | | |
| Pages | | | | |
| Groups CRUD | | | | |
| Members | | | | |
| Analytics | | | | |
| API | | | | |
| Errors | | | | |
| UI/UX | | | | |
| Browser | | | | |
| **Overall** | | | | |

---

## Sign-Off

**Tested By**: ___________________

**Date**: ___________________

**Status**: [ ] PASS [ ] FAIL [ ] NEEDS FIXES

**Comments**: ___________________________________________________________________________

---

**Next Steps**:
1. If PASS: Ready for production
2. If FAIL: File issues in issue tracker
3. If NEEDS FIXES: Review IMPLEMENTATION_SUMMARY.md for code details
