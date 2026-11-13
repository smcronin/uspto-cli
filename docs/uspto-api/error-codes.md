# Error Codes & Responses

## HTTP Error Codes

| Code | Error | Description |
|------|-------|-------------|
| **400** | Bad Request | Invalid query, filters, or request format |
| **403** | Forbidden | Authentication failure - invalid or missing API key |
| **404** | Not Found | No matching records found |
| **413** | Payload Too Large | Response exceeds 6 MB limit |
| **429** | Too Many Requests | Rate limit exceeded |
| **500** | Internal Server Error | Server-side error |

## Standard Error Response

```json
{
  "code": 400,
  "error": "Bad Request",
  "errorDetails": "Invalid request, review filter section...",
  "requestIdentifier": "07c5c24d-bf8e-458c-9427-a038500d6e98"
}
```

## 413 Payload Too Large Response

```json
{
  "code": 413,
  "message": "Payload Too Large",
  "detailedMessage": "Response payload exceeds allowed limit of 6MB...",
  "requestIdentifier": "uuid"
}
```

## Handling Strategies

### 400 Bad Request
- Check query syntax (field names are case-sensitive)
- Verify filter field names match schema
- Ensure date format is `yyyy-MM-dd`

### 403 Forbidden
- Verify API key is set in `X-API-KEY` header
- Check API key is valid and not expired

### 404 Not Found
- The application/trial/petition number may not exist
- Data coverage starts 2001-01-01 for patent applications

### 413 Payload Too Large
- Reduce `limit` parameter
- Use `fields` parameter to select fewer response fields
- Use download endpoint with pagination instead

### 429 Too Many Requests
- Wait at least 5 seconds before retrying (longer if `Retry-After` says so)
- Respect the ODP limits: one request at a time per API key (burst 1), with weekly quotas of 5M metadata calls and 1.2M document calls; see [rate-limits.md](./rate-limits.md)
- The per-minute tiers (60/120 requests, 4/12 PDF/ZIP downloads) are TSDR trademark API limits and do not apply here; see [../tsdr-api/operations.md](../tsdr-api/operations.md)

### 500 Internal Server Error
- Retry after a delay
- If persistent, check USPTO status or contact Help Desk

