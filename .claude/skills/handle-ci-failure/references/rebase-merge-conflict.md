# Rebase merge conflict

The rebase step of a sync workflow hit a merge conflict and bailed out. The rebase did not complete, so the working branch on `origin` is usually untouched (the workflow aborts the partial rebase before exiting). Your job is to reproduce the conflict locally, resolve it, finish the rebase, and push.

## Recognizing this pattern

Signals that route here:

- `workflow` is a sync or preview workflow (see `CI*.md` for names).
- `failed_step` contains "rebase".
- `log_tail` includes one of:
  - `CONFLICT (content): Merge conflict in <path>`
  - `CONFLICT (modify/delete): ... deleted in HEAD`
  - `error: could not apply <sha>... <subject>`
  - `hint: Resolve all conflicts manually`
- No annotations on source files (the failure is in the rebase process, not in compiled code).

If you see compile errors or test failures instead, you're in the wrong pattern — switch to `post-rebase-check-failure.md`.

## Fix recipe

1. Confirm local state matches what the workflow tried to rebase:

```bash
git fetch origin upstream
git log --oneline origin/<branch>..<branch>   # local-only commits
git log --oneline <branch>..origin/<branch>   # remote-only commits
```

(Branch name and upstream remote are in `CI*.md`.)

2. Reproduce the conflict:

```bash
git switch <branch>
git rebase <upstream-base>
```

Rebase will stop on the same commit the workflow stopped on, with the same conflicts.

3. For each conflicted file, understand both sides:

```bash
git diff --ours <file>      # what our commit tried to apply
git diff --theirs <file>    # what upstream did
git log --oneline <upstream-base> -- <file>    # recent upstream changes
git log --oneline <our-commit> -- <file>       # our changes to this file
```

**Auto-generated files:** Never resolve by hand. Check `CI*.md` for the project's regeneration command. The general pattern is:

```bash
# Run the rebase taking the upstream version on any conflict
git rebase -X ours <upstream-base>
# Regenerate the file to restore project-local additions
<regeneration command from CI*.md>
if ! git diff --quiet <file>; then
  git add <file>
  git commit --amend --no-edit
fi
```

**All other files:** Resolve by combining intent, not just by picking a side:

- If upstream moved or renamed the code we were patching, port the change to the new location.
- If upstream already made an equivalent change, drop our version (favor upstream, reduce divergence).
- If upstream removed the surface we were patching, evaluate whether the intent still applies in the new layout; re-implement if it does, drop it if not.

4. After resolving each file, stage and continue:

```bash
git add <resolved-file>
git rebase --continue
```

Repeat steps 3–4 for each conflicting commit until the rebase completes.

5. Verify:

```bash
<check command from CI*.md>
```

If this fails, you now have a post-rebase check failure — switch to `post-rebase-check-failure.md` and continue there before pushing.

6. Push (no new commit to author — rebase replayed existing commits with new SHAs):

```bash
git push --force-with-lease origin <branch>
```

## Red flags (stop, don't auto-resolve)

- The conflict is non-textual: binary files, generated artifacts, vendored dependencies. Resolution needs human judgment about which version should win.
- The conflict spans a large semantic restructure (a helper's signature changed, and resolving means rewriting how we use it). At that point it's a redesign, not a merge conflict.
- `git rebase --abort` has already been run and the working tree has unrelated changes. Clean up first.
- Repeated runs produce different conflicts — upstream is moving faster than the sync cadence. Consider rebasing more often instead of fighting bigger conflicts.
