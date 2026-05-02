---
name: handle-ci-failure
description: "TRIGGER immediately — before any manual investigation — whenever the user mentions CI failures: 'CI is failing', 'why is CI red?', 'the build is broken', 'CI failed', a run ID, a GitHub Actions URL, or any question about a failed workflow run. Do not investigate manually first. This skill reads project-local CI context (CI*.md), diagnoses the failure, matches it to a known pattern, applies the fix, verifies, and commits + pushes. Targets mechanical CI failures — not open-ended debugging."
argument-hint: "[run-id | run-url]"
---

# Handle CI failure

## Project CI context

!`cat CI*.md 2>/dev/null || echo "No CI*.md found — using defaults: branch from git, check command: make check, push: --force-with-lease."`

## Inputs

The user may provide:

- A bare run ID (`24375242526`).
- A `View results:` URL from a GitHub notification email — extract the numeric run ID with `grep -oE 'actions/runs/[0-9]+' <<<"$input" | cut -d/ -f3`.
- Nothing — list recent failures on the working branch and ask.

## Workflow

### 1. Resolve the run ID

If the user gave one, use it. Otherwise:

```bash
gh run list --branch <branch> --status=failure --limit 5
```

Show the list with run IDs and ages, and ask which one to handle. Don't silently pick "most recent" — it drifts between sessions.

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

Read the JSON once. Don't re-query `gh` for data the script already pulled.

### 3. Identify the real signal

`annotations` always contains runner-wrapper noise like `{path: ".github", message: "Process completed with exit code 1."}`. Filter `path != ".github"` when looking for root cause. Real signals point at real file paths with real messages. The `log_tail` is a fallback when annotations don't tell the whole story.

### 4. Match a known failure pattern

| Pattern                                                                                         | Reference                                  |
| ----------------------------------------------------------------------------------------------- | ------------------------------------------ |
| Rebase step itself failed (merge conflict during a sync or preview workflow)                    | `references/rebase-merge-conflict.md`      |
| Rebase succeeded, post-rebase check/test step failed (vet, lint, compile, tests)                | `references/post-rebase-check-failure.md`  |
| Check step failed with tool not found (e.g. `golangci-lint: No such file or directory`)        | `references/workflow-tool-missing.md`      |

Quick triage heuristic:

- Sync or preview workflow + `failed_step` mentions rebase, or `log_tail` shows conflict markers → rebase merge conflict.
- CI workflow (or post-rebase check step) + annotations point at source files with compile/lint/test errors → post-rebase check failure.
- Sync or preview workflow + `log_tail` shows `No such file or directory` or `command not found` for a tool → workflow tool missing.

If none fits, stop and report the diagnosis. This skill is for mechanical, well-understood failure classes — not open-ended debugging. Adding a new pattern is cheap (drop a file in `references/`, add a row here) but should be done deliberately after seeing the failure more than once.

### 5. Verify

Run the project's check command (see CI*.md; default `make check`). Must be green before committing. If it isn't, the fix is incomplete — don't paper over with `// nolint` or by deleting tests.

### 6. Commit

Confirm the working branch matches what CI*.md specifies (`git branch --show-current`). If it doesn't, stop.

Follow the project's commit style: terse but informative, prose for a single complex fix, bullets for multiple distinct changes. Explain why, not what.

Stage explicitly — `git add -u` for tracked files, `git add <path>` for new ones. Never `git add -A`. Before committing, confirm `CLAUDE.md` and `CLAUDE.local.md` are not staged.

### 7. Push

```bash
git push --force-with-lease origin <branch>
```

Use `--force-with-lease` by default; use plain `--force` only if the remote was updated by an automated workflow between your fetch and your push (confirm this is the case before overriding).

### 8. Confirm green

```bash
gh run list --branch <branch> --limit 3
```

Watch the new run with `gh run watch <run-id> --exit-status`. If it fails again with a different error, start over at step 1.

## Guardrails

- Never commit `CLAUDE.md` or `CLAUDE.local.md`.
- Never pass `--no-verify` or skip signing.
- Never use `git add -A` or `git add .`. Always stage explicitly.
- Never amend a pushed commit. New fix → new commit.
- Never force-push to a protected upstream branch (e.g. `master`, `main`).

## When to stop and escalate

- Failure doesn't match a pattern in the table above.
- Local reproduction gives a *different* error than CI reported — local state diverged; investigate before fixing.
- Check command won't go green after what looks like the right fix — don't commit a partial fix. Report what's left.
- Multiple jobs failed for unrelated reasons — handle them one at a time.
