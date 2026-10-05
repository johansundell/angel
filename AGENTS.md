# AGENTS.md

## Agent skills

### Issue tracker

GitHub Issues via `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical roles mapped 1:1 (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context (`CONTEXT.md` + `docs/adr/`). See `docs/agents/domain.md`.

## Workflow & PR Rules

- **Branch per ticket**: Every ticket must be implemented in a dedicated branch created from `main` (e.g. `feat/issue-<n>-<short-description>`).
- **Never commit directly to `main`**: All changes must go through a pull request.
- **Pull Request creation**: Once the implementation is complete, tests pass (`go test ./...`), and code review is satisfied, push the branch and open a PR via `gh pr create` referencing the issue (e.g. `Closes #<n>`).
