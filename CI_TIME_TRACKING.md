# CI context — fizzy-cli time-tracking fork

Read by the `handle-ci-failure` skill before pattern matching.

## Branch and remotes

- Working branch: `time-tracking`
- Upstream remote: `upstream`, base: `upstream/master`
- Rebase is a normal operation — `time-tracking` is force-pushed on every sync cycle. Use `--force-with-lease` when pushing fixes.

## Check command

```bash
make check   # fmt-check + vet + golangci-lint + tidy-check + race-test
```

`golangci-lint` must be installed. In CI it is installed via `golangci/golangci-lint-action` immediately before the `make check` step (see `fork-sync.yml` and `fork-preview.yml`). Locally it is expected to already be on PATH.

## Workflows

| Workflow       | Trigger       | What it does                                              |
| -------------- | ------------- | --------------------------------------------------------- |
| `fork-sync`    | Every 4h      | Syncs master from upstream, rebases time-tracking, releases |
| `fork-preview` | Every 12h     | Dry-run rebase only — no push, no release                 |

Both use `git rebase -X ours upstream/master` and regenerate `SURFACE.txt` afterwards.

## Known recurring conflict: SURFACE.txt

`SURFACE.txt` is auto-generated from the registered CLI command tree. It must never be resolved by hand. Resolution recipe:

```bash
git rebase -X ours upstream/master
GENERATE_SURFACE=1 go test ./internal/commands/ -run TestGenerateSurfaceSnapshot -v
if ! git diff --quiet SURFACE.txt; then
  git add SURFACE.txt
  git commit --amend --no-edit
fi
```

## Upstream tracking policy

Following `upstream/master` until upstream publishes a stable release post-v3.0.3. `v4.0.0-rc.1` exists but is a release candidate — not stable. Fork releases carry rc-based version strings (e.g. `4.0.0-rc.1-tt.N.gSHA`); this is expected. When a stable release appears, switch `fork-sync.yml` to rebase onto that tag instead.

Check: `gh release list --repo basecamp/fizzy-cli --limit 5`
