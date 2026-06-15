# CI failure handling context — fizzy-cli time-tracking fork

Read by the `handle-ci-failure` skill before pattern matching. Declares the branch, the commands
the skill runs, and the failure patterns specific to this fork.

## Branch and remotes

- Working branch: `time-tracking`
- Upstream remote: `upstream`, base: the latest stable release tag (see Upstream tracking policy)
- Rebase is a normal operation — `time-tracking` is force-pushed on every sync cycle. Push fixes
  with `--force-with-lease` (this is one of the few repos where force-pushing the working branch is
  expected).

## Commands

| Purpose                  | Command                                                                          |
| ------------------------ | -------------------------------------------------------------------------------- |
| Check (verify the tree)  | `make check` (fmt-check + vet + golangci-lint + tidy-check + race-test)          |
| Fix formatting           | `make fmt` (`gofmt -s -w .`)                                                      |
| Tidy dependencies        | `make tidy` (`go mod tidy`)                                                       |
| Regenerate `SURFACE.txt` | `make surface-snapshot` (`GENERATE_SURFACE=1 go test ./internal/commands/ -run TestGenerateSurfaceSnapshot -v`) |

`golangci-lint` must be on PATH. In CI it is installed via `golangci/golangci-lint-action`
immediately before the `make check` step (see `fork-sync.yml` and `fork-preview.yml`). Locally it
is expected to already be on PATH.

## Generated / checked-in files that drift

- `SURFACE.txt` — auto-generated from the registered CLI command tree; never hand-edit. This is the
  project's instance of the generic "regenerate and commit" pattern: regenerate with the command
  above, then commit. When the drift surfaces during a rebase rather than a check step, follow the
  amend recipe in the rebase pattern below.

## Workflows

| Workflow       | Trigger   | What it does                                                |
| -------------- | --------- | ----------------------------------------------------------- |
| `fork-sync`    | Every 4h  | Syncs master from upstream, rebases time-tracking, releases |
| `fork-preview` | Every 12h | Dry-run rebase only — no push, no release                   |

Both use `git rebase -X ours --onto <release-tag> upstream-base` and regenerate `SURFACE.txt`
afterwards.

`.github/FORK_AUTOMATION.md` covers what the automation needs to run: the `SYNC_TOKEN` permissions
and rotation, why a PAT is required at all, the `Sync` environment, and two traps worth knowing
before editing the fork workflows.

## Project failure patterns

Beyond the generic patterns the skill ships, match these fork-specific ones. Each points at a
project-local reference doc.

| Pattern                                                                          | Reference                                          |
| -------------------------------------------------------------------------------- | -------------------------------------------------- |
| Rebase step itself failed (merge conflict during a sync or preview workflow)     | `.github/ci-patterns/rebase-merge-conflict.md`     |
| Rebase succeeded, post-rebase check/test step failed (vet, lint, compile, tests) | `.github/ci-patterns/post-rebase-check-failure.md` |
| Sync workflow fails at `gh repo sync` with `HTTP 401: Bad credentials`           | `.github/FORK_AUTOMATION.md` — rotate the token    |

Triage between them: a sync/preview workflow with `failed_step` mentioning rebase, or `log_tail`
showing conflict markers → rebase merge conflict. A post-rebase check step with annotations on
source files (compile/lint/test errors) → post-rebase check failure. A missing tool in a fork
workflow → the generic `workflow-tool-missing.md`.

An expired `SYNC_TOKEN` needs no diagnosis beyond recognizing it, and cannot be fixed from here —
report it and stop. The tell is that it fails seconds into the run on the first step, while
`fork-preview` keeps passing because it needs no credentials. Since releases are downstream of the
sync's push, both stop together and nothing warns.

Local `golangci-lint` may be newer than the version CI pins, which makes `make check` stricter
locally than in CI. Before acting on a lint failure, confirm it reproduces in CI and that the file
is one the fork actually modifies — `git diff upstream-base HEAD -- <path>`. Lint failures on
files identical to upstream are not the fork's to fix.

## Upstream tracking policy

The fork rebases onto upstream's latest stable release, resolved from the releases API so that
release candidates are skipped. Fork versions read `<upstream-tag>-tt.N.gSHA`.

The rebase runs `--onto` against the `upstream-base` tag, which records the release the fork
currently sits on. A plain rebase onto a tag already reachable from `HEAD` does nothing and reports
success, so a sync that appears to work while leaving the fork on its old base points here.

Check: `gh release list --repo basecamp/fizzy-cli --limit 5`
