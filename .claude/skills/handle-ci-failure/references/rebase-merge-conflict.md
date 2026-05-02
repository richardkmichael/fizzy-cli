# Rebase merge conflict

The fork-sync (or fork-preview) workflow's `git rebase upstream/master` step hit a merge conflict and bailed out. The rebase did not complete, so `time-tracking` on `origin` is usually untouched (the workflow aborts the partial rebase before exiting). Your job is to reproduce the conflict locally, resolve it by understanding both sides of the diff, finish the rebase, and push the result.

## Recognizing this pattern

Signals that route here:

- `workflow` is `Fork sync` or `Fork preview`.
- `failed_step` contains "rebase" (e.g. `Rebase onto upstream master`).
- `log_tail` includes one of:
  - `CONFLICT (content): Merge conflict in <path>`
  - `CONFLICT (modify/delete): ... deleted in HEAD`
  - `error: could not apply <sha>... <subject>`
  - `hint: Resolve all conflicts manually`
- No annotations on source files (the failure is in the rebase process, not in compiled code).

If you see compile errors or test failures instead, you're in the wrong pattern — switch to `post-rebase-check-failure.md`.

## Fix recipe

1. Make sure your local `time-tracking` matches what the workflow tried to rebase. Since the workflow aborts on conflict, `origin/time-tracking` should not have advanced since the last successful sync — but confirm:

```bash
git fetch origin upstream
git status
git log --oneline origin/time-tracking..time-tracking     # local-only commits
git log --oneline time-tracking..origin/time-tracking     # remote-only commits
```

If there are unexpected divergences, reconcile them before rebasing — don't discard anything.

2. Reproduce the conflict:

```bash
git switch time-tracking
git rebase upstream/master
```

Rebase will stop on the same commit the workflow stopped on, with the same conflicts.

3. For each conflicted file, understand both sides of the diff:

```bash
git diff --ours <file>      # what our fork commit tried to apply
git diff --theirs <file>    # what upstream did
git log --oneline upstream/master -- <file>     # recent upstream changes to this file
git log --oneline <fork-commit> -- <file>       # fork's changes to this file
```

**Special case — `SURFACE.txt`:** This file is auto-generated from the registered command tree. Never resolve it by hand. Instead:

```bash
# Accept upstream's version (our fork entries will be regenerated below)
git checkout --theirs SURFACE.txt
git add SURFACE.txt
git rebase --continue   # repeat for any remaining commits that touch SURFACE.txt

# After the rebase completes, regenerate to restore fork-specific commands
GENERATE_SURFACE=1 go test ./internal/commands/ -run TestGenerateSurfaceSnapshot -v
git add SURFACE.txt
git commit --amend --no-edit   # fold into the last commit
```

Alternatively, run the entire rebase with `-X ours` (takes upstream's version on every conflict) and regenerate once at the end:

```bash
git rebase -X ours upstream/master
GENERATE_SURFACE=1 go test ./internal/commands/ -run TestGenerateSurfaceSnapshot -v
if ! git diff --quiet SURFACE.txt; then
  git add SURFACE.txt
  git commit --amend --no-edit
fi
```

For all other conflicted files, resolve by combining intent, not just by picking a side:

- If upstream moved or renamed the code the fork was patching, port the fork's change to the new location/name in the upstream version.
- If upstream already made an equivalent change, drop the fork's version (the fork's addition is now redundant — favor upstream).
- If upstream removed the surface the fork was patching (e.g., deleted a helper the fork's change added behavior to), the fork's change is orphaned. Evaluate whether the intent still applies in the new upstream layout; if it does, re-implement in the new shape, otherwise drop it.

Whenever you can resolve by *removing* fork-local code in favor of upstream, do. Minimum-divergence reduces future conflict surface.

4. After resolving each file, stage and continue:

```bash
git add <resolved-file>
git rebase --continue
```

Repeat steps 3–4 for each conflicting commit until the rebase completes.

5. Verify:

```bash
make check
```

If this fails, you now have a post-rebase check failure — switch to `post-rebase-check-failure.md` and continue there before pushing.

6. Return to SKILL.md step 6 (commit — but note: the rebase itself produced the "commit" by replaying; the next action is actually step 7, push). Specifically:

```bash
git push --force-with-lease origin time-tracking
```

No new commit to author — the rebase replayed the existing fork commits with new SHAs. The push itself is what unblocks the next sync cycle.

## Red flags (stop, don't auto-resolve)

- The conflict is non-textual: binary files, generated artifacts, vendored dependencies. Resolution needs human judgment about which version should win.
- The conflict spans a large semantic restructure (a helper's signature changed, and resolving means rewriting how the fork uses it). At that point it's not a merge conflict you're resolving — it's a redesign, and the fork's diff-against-upstream should be reconsidered, not just patched through.
- `git rebase --abort` has already been run by you or the workflow, and the working tree has unrelated changes. Clean up first; resolving a conflict on top of dirty state risks losing work.
- Repeated runs produce different conflicts — usually means upstream is moving faster than the sync cadence can keep up with. Consider rebasing more often instead of fighting bigger conflicts.
