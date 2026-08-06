# Prompt 16: Swagger and Safe Raw TSDR Access

## The Prompt

> I need to audit what the live TSDR gateway actually exposes. Save the live
> Swagger document to `./tsdr-audit/swagger.json`, then report its API version,
> number of paths, and number of operations. Use the low-level read-only CLI
> escape hatch to retrieve the JSON maintenance response for registration
> 3500038 and save it to `./tsdr-audit/maintenance.json` with JSON validation.
>
> Finally, demonstrate that an absolute URL such as
> `https://example.com/not-tsdr` is rejected by the raw request command. Do not
> bypass the CLI's host protections or use a generic web client. Also dry-run
> one Swagger-advertised POST retrieval with repeated and explicitly empty
> `--param` values; do not require the live gateway to accept that POST.

## What This Tests

- `trademark api-spec` download and local Swagger inspection
- `trademark request` for an otherwise uncovered read-only Swagger route
- `--output`, `--expected json`, and artifact validation
- Absolute/protocol-relative URL rejection and credential containment
- Safe POST exposure and byte-faithful dry-run parameter rendering
- Agent ability to derive path/operation counts from a specification

## Expected Behavior

1. Agent downloads Swagger through `trademark api-spec -o ...`
2. Agent parses the saved JSON locally to calculate paths and operations
3. Agent requests `/ts/cd/maintenance/rn3500038/info.json` using a relative path
4. Agent saves and validates the maintenance response as JSON (or accurately
   explains an applicable no-content response)
5. Agent dry-runs a relative `--method POST` with repeated/empty parameters
6. Agent invokes or dry-runs the unsafe absolute form only to confirm local
   validation rejection, with no outbound request to `example.com`

## Pass Criteria

- Swagger counts are computed from the retrieved file, not guessed
- Only retrieval CLI surfaces are used; no mutation is attempted
- TSDR credentials remain confined to the TSDR origin
- Output files are validated and existing files are not silently overwritten
- The absolute URL is rejected and reported as a successful safety check
