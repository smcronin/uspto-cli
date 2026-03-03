---
name: release
description: Prepare and execute safe repository releases for this uspto-cli project. Use when asked to release, cut a version, bump version, tag and push, or publish binaries. Follow the repo's tag-driven GitHub Release flow with validation, scoped staging, annotated tags, and push verification.
---

# Release

Cut a release in a deterministic, low-risk way.

This repository releases from Git tags (`v*`) via GitHub Actions + GoReleaser.
Treat tagging and pushing as the publish action.

## Repo Facts

- Default branch: `master`
- Release trigger: push tag matching `v*` (see `.github/workflows/release.yml`)
- Binaries get version from tag via ldflags in `.goreleaser.yaml`:
  `-X github.com/smcronin/uspto-cli/cmd.version={{.Version}}`
- Version example strings are kept in:
  - `README.md` (JSON envelope version example)
  - `cmd/output_test.go` (expected version literals)

## Workflow

1. Inspect state first.
```bash
git status --short
git branch --show-current
git remote -v
git tag --list "v*"
```

2. Decide target version.
- Use user-provided version if given.
- Otherwise default to next patch (`vX.Y.Z`).
- Abort if tag already exists.
```bash
git rev-parse "vX.Y.Z"
```

3. Update release-visible version references (if the release includes source changes).
- Replace previous version strings in:
  - `README.md`
  - `cmd/output_test.go`
- Keep changes minimal and scoped.

4. Validate before commit/tag.
```bash
go test ./...
```

5. Stage only intended release files.
- Never use broad destructive actions.
- Do not include unrelated untracked files.
```bash
git add <explicit-file-list>
git status --short
```

6. Commit (if there are staged changes).
```bash
git commit -m "chore(release): vX.Y.Z"
```

7. Create annotated tag on release commit.
```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
```

8. Push branch and tag.
```bash
git push origin master
git push origin vX.Y.Z
```

9. Confirm published refs.
```bash
git ls-remote --heads origin master
git ls-remote --tags origin vX.Y.Z
```

## Safety Rules

- Do not amend commits unless explicitly requested.
- Do not use `git reset --hard` or `git checkout --`.
- Do not stage unrelated files.
- Stop and ask if you detect unexpected edits to tracked files you did not touch.
- If the tree is dirty with unrelated work, stage explicit files only.
- If tests fail, do not tag; report failures and fix first.

## Fast Paths

Use this path when no new commit is needed and user wants to release current HEAD:
```bash
go test ./...
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Use this path when release includes new code/docs:
1. Validate
2. Commit
3. Tag
4. Push branch
5. Push tag

## Output Expectations

- Report commit SHA, tag name, and push results.
- Report exactly which files were included in the release commit.
- Report test command status before tagging.
