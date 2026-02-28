# API Rate Limits

Rate limiting is enforced **per API key** across all endpoints. There is no peak/off-peak distinction.

## Core Limits

| Limit Type | Value | Notes |
|-----------|-------|-------|
| **Burst** | **1** | NO parallel requests per API key. Must wait for each request to complete before sending the next. |
| **Rate** | **4-15 req/sec** | Depends on the call type |
| **Meta data APIs** | **5M calls/week** | Resets Sunday midnight UTC |
| **Document APIs** | **1.2M calls/week** | Resets Sunday midnight UTC |
| **Bulk downloads** | **5 files per 10 sec per IP** | Max 20 downloads/year per key (except XML bulk data) |

## 429 Handling

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

**Wait at least 5 seconds** before retrying after a 429.

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
- Enforces sequential requests (burst limit = 1)
- 100ms minimum gap between requests
- Automatic 5-second backoff on 429 responses
- All requests (metadata + downloads) go through the same rate limiter
