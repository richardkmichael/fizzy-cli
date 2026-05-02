---
name: handle-ci-failure
description: "TRIGGER immediately — before any manual investigation — whenever the user mentions CI on this fork: 'CI is failing', 'why is CI red?', 'the build is broken', 'CI failed', a run ID, a GitHub Actions URL, or any question about a failed workflow run on `time-tracking`. Do not investigate manually first. This skill diagnoses the failure, matches it to a known pattern, applies the fix, verifies with `make check`, and commits + force-pushes. Targets mechanical fork-maintenance failures — not open-ended debugging."
argument-hint: "[run-id | run-url]"
---

# Handle CI failure

Scope: this skill operates on the fork's `time-tracking` branch only. If the failing run is on a different branch, stop and report.

## Inputs

The user may provide:

- A bare run ID (`24375242526`).
- A `View results:` URL from a GitHub notification email
  - Extract the numeric run ID from a URL with `grep -oE 'actions/runs/[0-9]+' <<<"$input" | cut -d/ -f3`.
- Nothing — list recent failures on `time-tracking` and ask.

## Workflow

### 1. Resolve the run ID

If the user gave one, use it. Otherwise:

```bash
gh run list --branch time-tracking --status=failure --limit 5
```

Show the list with run IDs and ages, and ask which one to handle. Don't silently pick "most recent" — it drifts between sessions, and picking the wrong failure wastes more time than asking does.

### 2. Extract failure context

```bash
"$CLAUDE_SKILL_DIR"/scripts/ci-failure-context.sh <run-id>
```

Output JSON shape:

```json
{
  "run_id": "...", "run_url": "...", "workflow": "...",
  "branch": "...", "head_sha": "...", "conclusion": "failure",
  "failed_jobs": [
    {
      "id": 123, "name": "lint", "url": "...",
      "failed_step": "Run go vet",
      "annotations": [{"path": "...", "line": 15, "level": "failure", "message": "..."}],
      "log_tail": "... last 40 lines ..."
    }
  ]
}
```

Read the JSON once. Don't re-query `gh` for data the script already pulled — it wastes tool calls and risks reasoning about a stale view.

### 3. Identify the real signal

`annotations` always contains runner-wrapper noise like `{path: ".github", message: "Process completed with exit code 1."}`. Filter `path != ".github"` when looking for root cause. Real signals point at real file paths with real messages. The `log_tail` is a fallback when annotations don't tell the whole story.

### 4. Match a known failure pattern

Match the failure against the table below. Each pattern has a dedicated reference file with its fix recipe; read only the one that matches.

| Pattern                                                                                    | Reference                                  |
| ------------------------------------------------------------------------------------------ | ------------------------------------------ |
| Rebase step itself failed (merge conflict during fork-sync / fork-preview rebase)          | `references/rebase-merge-conflict.md`      |
| Rebase succeeded, post-rebase `make check` / test step failed (vet, lint, compile, tests) | `references/post-rebase-check-failure.md`  |
| `make check` step failed with tool not found (e.g. `golangci-lint: No such file or directory`) | `references/workflow-tool-missing.md`  |

Quick triage heuristic:

- `workflow` is `Fork sync` or `Fork preview` and `failed_step` mentions rebase, or `log_tail` shows conflict markers → rebase merge conflict.
- `workflow` is `CI` (or `Fork preview` post-rebase step) and annotations point at source files with compile/lint/test errors → post-rebase check failure.
- `workflow` is `Fork sync` or `Fork preview` and `log_tail` shows `No such file or directory` or `command not found` for a tool → workflow tool missing.

If neither fits, stop and report the diagnosis. This skill is for mechanical, well-understood failure classes — not open-ended debugging. Adding a new pattern is cheap (drop a file in `references/`, add a row here) but should be done deliberately after seeing the failure more than once.

### 5. Verify

```bash
make check
```

Runs fmt-check, vet, lint, race tests, and tidy-check. Must be green before committing. If it isn't, the fix is incomplete — don't paper over with `// nolint` or by deleting tests. For non-trivial changes, also run affected unit tests explicitly.

### 6. Commit

Before committing, confirm the working branch is `time-tracking` (`git branch --show-current`). If it isn't, stop — don't commit the fix to a different branch.

Follow the project's commit style: terse but informative, prose for a single complex fix, bullets for multiple distinct changes. Explain why, not what. Include the `Co-Authored-By:` trailer matching the format of recent commits on this branch.

Stage explicitly — `git add -u` for tracked files, `git add <path>` for new ones. Never `git add -A`. Before committing, confirm `CLAUDE.md` and `CLAUDE.local.md` are not in the staged set.

### 7. Push

```bash
git push --force-with-lease origin time-tracking
```

Force-push is normal on `time-tracking` — the fork-sync workflow force-pushes on every rebase, so the branch is not append-only. `--force-with-lease` protects against racing the scheduled sync: if the workflow rebased between your fetch and your push, the push fails and you re-investigate before retrying.

### 8. Confirm green

```bash
gh run list --branch time-tracking --limit 3
```

The push triggers a new CI run. Watch it with `gh run watch <run-id> --exit-status`, or hand the user the URL. If it fails again with a different error, start over at step 1. If it fails with the same error, the fix didn't actually address the root cause.

## Guardrails

- Never commit `CLAUDE.md` or `CLAUDE.local.md`.
- Never pass `--no-verify` or skip signing. If a hook fails, investigate the complaint — don't route around it.
- Never use `git add -A` or `git add .`. Always stage explicitly.
- Never amend a pushed commit. New fix → new commit.
- Never force-push to `master` or any non-fork branch.
- If the failing run is on any branch other than `time-tracking`, stop.

## When to stop and escalate

- Failure doesn't match a pattern in the table above.
- Local rebase reproduces a *different* error than CI reported — your local state diverged; investigate before fixing.
- `make check` won't go green after what looks like the right fix — don't commit a partial fix to "unblock CI". Report what's left.
- Multiple jobs failed for unrelated reasons — handle them one at a time, not as a bundle.
