# Getting USPTO API Keys

The CLI talks to three USPTO services. Patent ODP and trademark TSDR use different accounts, hosts, header names, and keys; Trademark Search currently requires no key.

| What you want to do | Service | Key source | CLI setting |
| --- | --- | --- | --- |
| Search patents, retrieve patent file wrappers, use PTAB/petitions, or discover bulk products | Open Data Portal (ODP) | [MyODP](https://data.uspto.gov/myodp) | `USPTO_API_KEY` |
| Search marks by wording, owner, goods/services, class, or status | [Trademark Search](https://tmsearch.uspto.gov/) companion service | No key currently required | None |
| Retrieve official trademark case status, documents, images, or other TSDR artifacts | Trademark Status and Document Retrieval (TSDR) | [USPTO API Manager](https://account.uspto.gov/api-manager/) | `USPTO_TSDR_API_KEY` |

An ODP key does **not** authenticate at `tsdrapi.uspto.gov`, and a TSDR key should never be sent to `api.uspto.gov`.

## Patent Open Data Portal Key

### 1. Create an account

Open the [ODP getting-started page](https://data.uspto.gov/apis/getting-started) and sign in or create a MyUSPTO account.

### 2. Complete identity verification

Complete the one-time ID.me verification required for ODP API-key access.

### 3. Copy the ODP key

Open the [MyODP dashboard](https://data.uspto.gov/myodp) and copy the key shown there. The CLI sends it only to the configured ODP origin using:

```http
X-API-KEY: <ODP key>
```

Configure this key with `uspto config set-api-key` or `USPTO_API_KEY`.

## Trademark TSDR Key

### 1. Open USPTO API Manager

Sign in to the separate [USPTO API Manager](https://account.uspto.gov/api-manager/) and obtain a key for the Trademark Status and Document Retrieval API.

### 2. Configure it separately

The CLI sends the TSDR credential only to the configured `tsdrapi.uspto.gov` origin using the exact header required by the official guide:

```http
USPTO-API-KEY: <TSDR key>
```

Configure this key with `uspto config set-tsdr-api-key` or `USPTO_TSDR_API_KEY`. The legacy name `TSDR_API_KEY` remains accepted for compatibility, but new setups should use `USPTO_TSDR_API_KEY`.

If a TSDR command has no TSDR key, the CLI stops before making the request and points to API Manager. It never falls back to `USPTO_API_KEY`.

## Trademark Search Needs No Key

`uspto trademark search` uses the public companion backend currently used by the official Trademark Search web UI. It does not send either API key.

```bash
uspto trademark search --wordmark "OPENAI" --status live --limit 10
```

USPTO has not published this backend as a stable developer contract. The CLI discovers its current service URL at runtime, but agents should still treat search results as candidates and retrieve selected official records through TSDR. See [service boundaries](tsdr-api/service-boundaries.md).

## Configure the CLI

### Recommended: import from environment

This avoids placing a key directly in a `uspto` command line. In Bash, zsh, or a similar shell:

```bash
export USPTO_API_KEY="YOUR_ODP_KEY"
export USPTO_TSDR_API_KEY="YOUR_TSDR_KEY"

uspto config set-api-key --from-env
uspto config set-tsdr-api-key --from-env
uspto config show
```

In PowerShell:

```powershell
$env:USPTO_API_KEY = "YOUR_ODP_KEY"
$env:USPTO_TSDR_API_KEY = "YOUR_TSDR_KEY"

uspto config set-api-key --from-env
uspto config set-tsdr-api-key --from-env
uspto config show
```

The setters write both values to the same user-level config without replacing the other credential. `config show` reports both slots with masked values.

Typical config locations are `%AppData%\uspto\config.env` on Windows and `~/.config/uspto/config.env` on Linux. The path returned by `uspto config show` is authoritative for the current machine.

### Import from a dotenv file

Create a local file that is excluded from version control:

```dotenv
USPTO_API_KEY="YOUR_ODP_KEY"
USPTO_TSDR_API_KEY="YOUR_TSDR_KEY"
```

Then import either or both credentials:

```bash
uspto config set-api-key --from-dotenv .env
uspto config set-tsdr-api-key --from-dotenv .env
```

The CLI does not need to keep reading that project file after import. Never commit the dotenv file.

### Persist only in the shell

You may leave the variables in your shell profile instead of importing them. This is useful on CI workers and ephemeral machines:

```bash
export USPTO_API_KEY="YOUR_ODP_KEY"
export USPTO_TSDR_API_KEY="YOUR_TSDR_KEY"
```

### Pass a key for one command

The global flags are available when an ephemeral override is necessary:

```bash
uspto search --api-key "YOUR_ODP_KEY" --title "machine learning"
uspto trademark case status --tsdr-api-key "YOUR_TSDR_KEY" sn:97054561
```

Command-line values may be retained in shell history or process listings, so prefer environment or global config for routine use.

## Credential Precedence

For ODP commands, the CLI resolves the key in this order:

1. `--api-key`
2. `USPTO_API_KEY`
3. The user-level `uspto` config file

For TSDR commands, it resolves:

1. `--tsdr-api-key`
2. `USPTO_TSDR_API_KEY`
3. Legacy `TSDR_API_KEY`
4. The user-level `uspto` config file

`trademark search`, `config`, `update`, help, and version commands do not require either credential.

## Key Safety and Policies

- Do not commit keys, paste them into issues, place them in URLs, or include them in debug logs.
- Do not share keys. Each user should obtain credentials under their own USPTO account and follow the terms for that service.
- ODP permits one key per verified user and does not issue organization-wide shared keys. Consult the [ODP FAQ](https://data.uspto.gov/support/faq) for current policies and expiration rules.
- Keep ODP and TSDR credentials scoped to their own hosts. The CLI removes the TSDR header before following a cross-host redirect.
- TSDR quotas apply per key, not per process. Coordinate agents and prefer metadata before PDF/ZIP downloads.
- Rotate a key through its issuing portal if it may have been exposed, then update the corresponding CLI config slot.

## Rate Limits

TSDR publishes 60 total requests per key per minute during peak hours and 120 off-peak. PDF/ZIP requests are limited to 4 per minute peak and 12 off-peak. The CLI applies conservative cross-process spacing automatically; see [TSDR authentication, limits, and retries](tsdr-api/operations.md).

ODP uses separate weekly metadata and document quotas; see the [ODP rate-limit reference](uspto-api/rate-limits.md).

## Official Links

- [ODP Getting Started](https://data.uspto.gov/apis/getting-started)
- [MyODP Dashboard](https://data.uspto.gov/myodp)
- [ODP API Documentation](https://data.uspto.gov/apis)
- [ODP FAQ](https://data.uspto.gov/support/faq)
- [USPTO API Manager for TSDR](https://account.uspto.gov/api-manager/)
- [Official TSDR FAQ](https://tsdr.uspto.gov/faqview)
- [USPTO Trademark Enterprise API User Guide](https://www.uspto.gov/sites/default/files/documents/tm-enterprise-api-user-guide-v2.pdf)
- [Trademark Search](https://tmsearch.uspto.gov/) and [search help](https://tmsearch.uspto.gov/?page=help)
- [Repository TSDR reference](tsdr-api/README.md)
