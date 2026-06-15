# CI failure handling context — fizzy-cli time-tracking fork

Read by the `handle-ci-failure` skill before pattern matching. Declares the branch, the commands
the skill runs, and the failure patterns specific to this fork.

## Branch and remotes

- Working branch: `time-tracking`
- Upstream remote: `upstream`, base: `upstream/master`
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

Both use `git rebase -X ours upstream/master` and regenerate `SURFACE.txt` afterwards.

## Project failure patterns

Beyond the generic patterns the skill ships, match these fork-specific ones. Each points at a
project-local reference doc.

| Pattern                                                                            | Reference                                      |
| ---------------------------------------------------------------------------------- | ---------------------------------------------- |
| Rebase step itself failed (merge conflict during a sync or preview workflow)       | `.github/ci-patterns/rebase-merge-conflict.md`     |
| Rebase succeeded, post-rebase check/test step failed (vet, lint, compile, tests)   | `.github/ci-patterns/post-rebase-check-failure.md` |

Triage between them: a sync/preview workflow with `failed_step` mentioning rebase, or `log_tail`
showing conflict markers → rebase merge conflict. A post-rebase check step with annotations on
source files (compile/lint/test errors) → post-rebase check failure. A missing tool in a fork
workflow → the generic `workflow-tool-missing.md`.

## Upstream tracking policy

Following `upstream/master` until upstream publishes a stable release post-v3.0.3. `v4.0.0-rc.1`
exists but is a release candidate — not stable. Fork releases carry rc-based version strings (e.g.
`4.0.0-rc.1-tt.N.gSHA`); this is expected. When a stable release appears, switch `fork-sync.yml` to
rebase onto that tag instead.

Check: `gh release list --repo basecamp/fizzy-cli --limit 5`
