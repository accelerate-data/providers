# Providers (Accelerate fork)

Fork of `obot-platform/providers`: model and auth providers for the Obot MCP gateway. Each provider is its own Go module (e.g. `github-auth-provider/`, `generic-oauth-auth-provider/`, `openai-model-provider/`); the `Dockerfile` packages them into two images.

## CI Gates and Release Contract

This repo publishes two Studio release inputs:

- `ghcr.io/accelerate-data/providers-vibedata:latest`
- `ghcr.io/accelerate-data/providers-vibedata/encryption-bins:latest`

Studio's nightly release pipeline resolves the `:latest` digests at candidate time and pins them into the candidate tag (vd-studio `docs/functional/release-management/README.md`). The `accelerate-data/obot` image build also consumes both images as `FROM` inputs, pinned to verified digests.

### Required checks on `main`

The GitHub ruleset is versioned at `.github/rulesets/main-branch.json`. Create it with `gh api repos/accelerate-data/providers/rulesets --input .github/rulesets/main-branch.json`; update an existing one with `--method PUT` against `.../rulesets/<id>`. Required contexts:

- `go-test` (`test.yaml`) — Go tests for the auth provider modules.
- `docker-build` (`docker-build-check.yml`) — builds both Dockerfile targets (amd64, no push), the repo's real build contract.

Do not add paths filters to a required check's workflow — non-matching PRs could never merge.

Upstream-sync PRs are drafts created with `GITHUB_TOKEN`, whose events trigger no workflows — the required checks start when a maintainer marks the PR ready for review. If the branch is updated by automation after that, close and reopen the PR to re-trigger the checks.

### Image publication is fail-closed

`build-vibedata-providers.yml` (push to `main` or manual dispatch) is the only workflow that advances the `:latest` tags; the upstream `Build providers` / `Build release encryption bins` workflows are `workflow_dispatch`-only. Stage order: per-arch builds pushed by digest only (untagged) → manifest merge creates the `:latest` tags → both-arch verification → cosign signing. A failure at any stage leaves the previous `:latest` untouched.

If Studio picks up a broken or stale providers input, the owning failure is a red `Build and Push providers-vibedata` run on `main` in this repo.

### Local verification

- `make test` — Go tests for the auth providers
- `make docker-build` — builds the providers image target locally
- `docker build --target encryption-bins .` — builds the encryption-bins target
