# Event Service API Contract

**Base Path**: `/api/v1/events`

## Endpoints

### GET /events

List all upcoming events. Public endpoint (no authentication required).

**Query Parameters**:
- `page` (optional, default: 1): page number for pagination
- `per_page` (optional, default: 20, max: 100): items per page

**Response 200**:
```json
{
  "events": [
    {
      "id": 1,
      "name": "Summer Music Festival",
      "date": "2025-08-15T18:00:00Z",
      "venue": "Central Park Amphitheater",
      "remaining_count": 245,
      "sold_out": false
    },
    {
      "id": 2,
      "name": "Tech Conference 2025",
      "date": "2025-09-01T09:00:00Z",
      "venue": "Convention Center Hall B",
      "remaining_count": 0,
      "sold_out": true
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 45
  }
}
```

---

### GET /events/:id

Get detailed information for a specific event. Public endpoint.

**Response 200**:
```json
{
  "id": 1,
  "name": "Summer Music Festival",
  "description": "A full day of live music featuring top artists...",
  "date": "2025-08-15T18:00:00Z",
  "venue": "Central Park Amphitheater",
  "total_capacity": 500,
  "remaining_count": 245,
  "sold_out": false
}
```

**Response 404**:
```json
{
  "error": "Event not found"
}
```

---

## Authentication

Event browsing endpoints are public — no authentication required. This aligns with
FR-018 (open browsing for unauthenticated visitors).
