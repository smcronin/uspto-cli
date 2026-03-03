# Getting a USPTO API Key

The USPTO Open Data Portal requires an API key for all requests. Each user
gets one key, and keys must not be shared per USPTO policy.

## Step 1: Create a USPTO account

Go to the [USPTO Open Data Portal](https://data.uspto.gov/apis/getting-started)
and click **Sign In** to create a MyUSPTO account.

## Step 2: Verify your identity with ID.me

After creating your account, you must complete identity verification through
[ID.me](https://www.id.me/). This is a one-time requirement before you can
access API keys. The USPTO uses ID.me to verify that each API key belongs
to a real person.

## Step 3: Get your API key from MyODP

Once your identity is verified, sign in to the
[MyODP dashboard](https://data.uspto.gov/myodp). Your API key will be
displayed on the dashboard.

## Step 4: Configure the CLI

Recommended: store your key once in global CLI config:

```bash
uspto config set-api-key your-api-key-here
```

This writes your key to your user config directory (for example:
`%AppData%\uspto\config.env` on Windows, `~/.config/uspto/config.env`
on Linux/macOS), so `uspto` works from any directory.

This is runtime configuration only. Your API key is not embedded into
the binary when you build/package the CLI.

Alternative options:

```bash
# Set in shell profile (~/.bashrc, ~/.zshrc, PowerShell profile, etc.)
export USPTO_API_KEY=your-api-key-here

# Pass directly on each command
uspto search --api-key your-api-key-here --title "machine learning"
```

## Key policies

- **One key per user.** Each verified account receives a single API key.
- **No organization-wide keys.** Each person must have their own account and key.
- **Do not share keys.** USPTO terms prohibit sharing API keys between users.
  If multiple people need access, each must create their own account.
- **Keys do not expire** as long as they are used at least once per year.
- **Rate limits apply.** See the [rate limits documentation](uspto-api/rate-limits.md)
  for details on request quotas.

## Links

- [Getting Started](https://data.uspto.gov/apis/getting-started) — API overview and sign-in
- [MyODP Dashboard](https://data.uspto.gov/myodp) — View your API key
- [API Documentation](https://data.uspto.gov/apis) — Full endpoint reference
- [FAQ](https://data.uspto.gov/support/faq) — Common questions about API keys, rate limits, and data access


