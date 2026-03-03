# Contributing to uspto

Thanks for your interest. This is a small personal project, so contributions follow a different process than most open-source repos.

## The Rule

**Don't just push a PR. Send me the prompt.**

If you used an AI agent to generate your changes, I need to see the original prompt — the actual instructions you gave the agent. That tells me what you were *trying* to do, and I can decide if it fits the spirit of the project.

A diff tells me *what* changed. The prompt tells me *why* and *whether I agree with the intent*.

## How to Contribute

1. **Open an issue first.** Describe what you want to change and why.
2. **Include your prompt** if you used an AI agent to generate the code. Paste it in the issue or PR description.
3. **Wait for a response** before investing time in a PR. I may want to take a different approach.

## What I'll Look At

- Bug fixes with clear reproduction steps
- Missing API endpoint coverage
- Test coverage improvements
- Documentation fixes

## What I'll Probably Decline

- Large refactors without discussion
- New dependencies (this is a zero-dependency binary by design)
- Feature additions that don't map to a real USPTO API capability
- PRs without context on intent

## Development

```bash
go build -o uspto .
go vet ./...
go test ./...
go test ./tests/integration/ -v -count=1 -timeout 600s  # requires USPTO_API_KEY
```

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

