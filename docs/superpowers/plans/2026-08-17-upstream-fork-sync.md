# Providers Upstream Fork Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Synchronize `accelerate-data/providers` with `obot-platform/providers` at `1e126d06737ef5b9dc66c9ef0575ef2a25c81880`, retain the net Accelerate behavior, merge PR #8 with upstream ancestry, and verify the stable `v0.25.1` publication.

**Architecture:** Rebuild the existing PR branch from upstream `main`, apply the net fork delta since checkpoint `b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de`, and add only required fork-specific behavior. Keep build and sync-automation repairs in separate commits. Update the existing remote branch with force-with-lease; merge only through the protected branch with a merge commit.

**Tech Stack:** Git, GitHub Actions, Bash, Docker Buildx, Go 1.26

**Spec:** `docs/adr/0001-upstream-fork-synchronization.md`

## Global Constraints

- Upstream is canonical when it provides equivalent behavior.
- Exclude historical sync commits and `.accelerate/upstream-sync.json` from the replayed fork delta.
- Preserve undocumented unique behavior and flag it in PR #8.
- Use the highest stable upstream tag, `v0.25.1`, and record the exact upstream SHA separately.
- Never force-push fork `main`; update only `sync/upstream-20260817-1e126d0` with force-with-lease.
- Merge PR #8 with a merge commit after required checks pass.
- Verify image publication before starting the Obot synchronization.

---

### Task 1: Rebuild the providers sync branch from upstream

**Files:**
- Create: `docs/adr/0001-upstream-fork-synchronization.md`
- Create: `docs/superpowers/plans/2026-08-17-upstream-fork-sync.md`
- Modify: `.accelerate/upstream-sync.json`
- Retain net fork files from `b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de..origin/main`

**Interfaces:**
- Consumes: `upstream/main=1e126d06737ef5b9dc66c9ef0575ef2a25c81880`, `origin/main=ca8ca1c6cca1243164bfb94c1ba377d69485c2e3`
- Produces: local `sync/upstream-20260817-1e126d0` with upstream as its first ancestor and the net fork delta staged on top

- [ ] **Step 1: Refresh and verify immutable inputs**

Run:

```bash
git fetch --prune origin
git fetch --prune upstream --tags
git rev-parse origin/main upstream/main
git merge-base --is-ancestor b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de origin/main
git merge-base --is-ancestor b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de upstream/main
```

Expected: the two SHAs above and both ancestry checks exit 0.

- [ ] **Step 2: Rebuild the temporary branch from upstream**

Run the branch operation only after preserving the ADR and plan commit:

```bash
git switch -C sync/upstream-20260817-1e126d0 upstream/main
git diff --binary b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de origin/main -- . ':(exclude).accelerate/upstream-sync.json' | git apply --3way --index
```

Expected: the delta applies without conflicts, matching the disposable probe.

- [ ] **Step 3: Write current sync metadata**

Set `upstreamHeadSha` to `1e126d06737ef5b9dc66c9ef0575ef2a25c81880`, `mergeBaseSha` to `b7d03b574e3ce7dcc2978fa24f1f4e3d69d612de`, `syncBranch` to `sync/upstream-20260817-1e126d0`, and `syncTimestamp` to the current UTC timestamp.

- [ ] **Step 4: Commit the reconciled net fork behavior**

Group retained changes by provider behavior, release automation, and documentation. Do not commit generated sync metadata with product behavior.

### Task 2: Make the Docker interpreter contract executable

**Files:**
- Create: `scripts/verify-docker-build-contract.sh`
- Modify: `Dockerfile`
- Modify: `Makefile`

**Interfaces:**
- Consumes: executable scripts referenced by Docker build targets
- Produces: a structural test that rejects a missing Bash runtime and a Docker base that can execute `package-providers.sh`

- [ ] **Step 1: Add the failing structural test**

Create `scripts/verify-docker-build-contract.sh` to assert that `scripts/package-providers.sh` uses Bash and that the Docker base `apk add` command installs `bash`. Add it to `make test` before Go tests.

- [ ] **Step 2: Verify RED**

Run:

```bash
bash scripts/verify-docker-build-contract.sh
```

Expected: failure stating that the Docker base does not install Bash.

- [ ] **Step 3: Implement the minimal fix**

Add `bash` to the existing `apk add --no-cache go-1.26 make git curl` package list in `Dockerfile`.

- [ ] **Step 4: Verify GREEN and the real image contract**

Run:

```bash
bash scripts/verify-docker-build-contract.sh
docker buildx build --target providers --platform linux/amd64 --load -t providers-sync-providers .
docker buildx build --target encryption-bins --platform linux/amd64 --load -t providers-sync-encryption .
```

Expected: both builds exit 0.

- [ ] **Step 5: Commit the Docker repair**

Commit only `Dockerfile`, `Makefile`, and `scripts/verify-docker-build-contract.sh` as `fix(build): install the providers script interpreter`.

### Task 3: Make upstream-sync automation self-contained

**Files:**
- Create: `scripts/verify-upstream-sync-workflow.sh`
- Modify: `.github/workflows/upstream-sync.yml`
- Modify: `.github/workflows/build-vibedata-providers.yml`
- Modify: `Makefile`

**Interfaces:**
- Consumes: GitHub token permissions and repository labels
- Produces: a workflow that creates required labels before `gh pr create`, records the stable upstream version, avoids `pipefail`/`head` SIGPIPE, gives exact conflict-resolution instructions, and publishes matching versioned image tags

- [ ] **Step 1: Add the failing workflow contract test**

Assert that the workflow has `issues: write`, creates `upstream-sync`, `sync:clean`, `sync:touches-customized-files`, and `sync:conflicts` labels with `gh label create --force`, records `upstreamVersion`, uses `git log --max-count=50`, never pipes `git log` into `head`, and publishes both `latest` and `${upstreamVersion}-vibedata` tags for both images.

- [ ] **Step 2: Verify RED**

Run `bash scripts/verify-upstream-sync-workflow.sh`.

Expected: failure because the current workflow does not bootstrap labels and still uses `git log ... | head -50`.

- [ ] **Step 3: Implement the workflow contract**

Add a label-bootstrap step after checkout/configuration, resolve and record the highest stable upstream tag, replace the `head` pipeline with `--max-count=50`, document the exact checkout/fetch/merge commands in conflict PR bodies, and make publication resolve the version from metadata before atomically creating both tags.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
bash scripts/verify-upstream-sync-workflow.sh
make test
```

Expected: all structural and Go tests pass.

- [ ] **Step 5: Commit the automation repair**

Commit the sync workflow, publication workflow, structural test, and Makefile change as `fix(ci): make providers sync and publication self-contained`.

### Task 4: Publish, review, merge, and verify providers

**Files:**
- Modify: PR #8 title/body/branch
- Verify: `.accelerate/upstream-sync.json`

**Interfaces:**
- Consumes: fully verified local sync branch
- Produces: merged `origin/main` containing upstream `1e126d0`, passing release publication, and stable version `v0.25.1`

- [ ] **Step 1: Run the full local gate**

Run `make test`, both Docker target builds, and `git diff --check`.

- [ ] **Step 2: Publish with an exact lease**

Read the current remote branch SHA, then push with:

```bash
git push --force-with-lease=refs/heads/sync/upstream-20260817-1e126d0:a805f1727f41568ae9d52d1af421d49d8982528c origin HEAD:sync/upstream-20260817-1e126d0
```

- [ ] **Step 3: Update and ready PR #8**

Update its body with retained behavior, dropped duplicates, validation evidence, upstream SHA, and stable version. Mark it ready and wait for all required checks.

- [ ] **Step 4: Merge with a merge commit**

Run `gh pr merge 8 --repo accelerate-data/providers --merge --delete-branch=false` only after checks pass.

- [ ] **Step 5: Verify source and publication**

Fetch `origin/main`; prove `upstream/main` is an ancestor; verify `.accelerate/upstream-sync.json`; wait for `Build and Push providers-vibedata`; and confirm the `latest` and `v0.25.1-vibedata` manifests resolve to the synchronized source.
