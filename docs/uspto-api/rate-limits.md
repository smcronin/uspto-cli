# API Rate Limits

Rate limiting is enforced **per API key** across all endpoints.

## Peak Hours (5:00 AM - 10:00 PM EST, all 7 days)

| Request Type | Limit |
|-------------|-------|
| All API requests | **60 requests per minute** (~1 req/sec) |
| PDF, ZIP, and multi-case downloads | **4 requests per minute** |

## Off-Peak Hours (10:00 PM - 5:00 AM EST, all 7 days)

| Request Type | Limit |
|-------------|-------|
| All API requests | **120 requests per minute** (~2 req/sec) |
| PDF and ZIP downloads | **12 requests per minute** |

## Rate Limit Response

When rate limit is exceeded, the API returns:

```
HTTP/1.1 429 Too Many Requests
```

```json
{
  "code": 429,
  "error": "Too Many Requests",
  "errorDetails": "Rate limit exceeded",
  "requestIdentifier": "uuid"
}
```

## Response Payload Limit

Maximum response payload: **6 MB**

If a response would exceed 6 MB, the API returns HTTP 413:

```json
{
  "code": 413,
  "message": "Payload Too Large",
  "detailedMessage": "Response payload exceeds allowed limit of 6MB...",
  "requestIdentifier": "uuid"
}
```

## CLI Rate Limiting Strategy

The CLI implements automatic rate limiting:
- Tracks requests per minute
- Automatically throttles when approaching limits
- Uses exponential backoff on 429 responses
- Detects peak vs off-peak based on current EST time
- Download commands (PDF/ZIP) use stricter rate limiting
