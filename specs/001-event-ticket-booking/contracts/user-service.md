# User Service API Contract

**Base Path**: `/api/v1/users`

## Endpoints

### POST /register

Register a new user account.

**Request**:
```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "password": "securePassword123"
}
```

**Validation**:
- `name`: required, 1-255 chars
- `email`: required, valid email format, must be unique
- `password`: required, min 8 chars

**Response 201**:
```json
{
  "message": "Account created successfully",
  "user_id": 42
}
```

**Response 409** (duplicate email):
```json
{
  "error": "An account with this email already exists"
}
```

**Response 400** (validation error):
```json
{
  "error": "Password must be at least 8 characters"
}
```

---

### POST /login

Authenticate a user and return a session token.

**Request**:
```json
{
  "email": "jane@example.com",
  "password": "securePassword123"
}
```

**Response 200**:
```json
{
  "token": "sess_a1b2c3d4e5f6...",
  "user_id": 42,
  "expires_at": "2025-07-15T14:30:00Z"
}
```

**Response 401**:
```json
{
  "error": "Invalid email or password"
}
```

---

### POST /logout

Invalidate the current session token.

**Headers**: `Authorization: Bearer <token>`

**Request**: (empty body)

**Response 200**:
```json
{
  "message": "Logged out successfully"
}
```

**Response 401** (invalid/expired token):
```json
{
  "error": "Unauthorized"
}
```

---

### GET /me

Get the authenticated user's profile.

**Headers**: `Authorization: Bearer <token>`

**Response 200**:
```json
{
  "user_id": 42,
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2025-07-14T10:00:00Z"
}
```

**Response 401**:
```json
{
  "error": "Unauthorized"
}
```

---

## Authentication

All protected endpoints require header `Authorization: Bearer <session_token>`. Session tokens
are stored in Redis with a 24-hour TTL. Each authenticated request refreshes the TTL.
