# Authentication

## API Key Authentication

All requests to the USPTO ODP API require an API key passed via HTTP header.

| Property | Value |
|----------|-------|
| **Scheme** | `ApiKeyAuth` |
| **Type** | API Key |
| **Header Name** | `X-API-KEY` |
| **Location** | HTTP Request Header |

### Example Request

```bash
curl -H "X-API-KEY: your-api-key-here" \
  "https://api.uspto.gov/api/v1/patent/applications/search?q=inventionTitle:wireless&limit=5"
```

## Obtaining an API Key

1. Go to https://data.uspto.gov/apis/getting-started
2. Create a USPTO.gov account (requires ID.me identity verification)
3. After login, navigate to "My ODP" page
4. View/copy your API key

## Key Details

- The API key is both a unique identifier and a secret token
- It is linked to your specific user account
- Keep it private - do not commit to version control
- Rate limits are enforced per API key

## Environment Variables

Convention used by client libraries:
- `USPTO_API_KEY` (this CLI)
- `PATENT_CLIENT_ODP_API_KEY` (Python patent_client library)

## Legacy Header

The older TSDR API used `USPTO-API-KEY` as the header name. The current ODP API uses `X-API-KEY`.

## Sandbox

A sandbox environment is available at `https://sandbox-api.uspto.gov` for testing.

