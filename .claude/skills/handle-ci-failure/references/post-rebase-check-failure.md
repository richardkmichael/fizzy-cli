# Post-rebase check failure

The rebase onto upstream `master` completed cleanly, but the resulting tree fails `make check`. This usually means upstream restructured code the fork depends on — helpers moved, packages renamed, types removed — and the fork's commits replayed on top still reference the old identities. The fix is always: adapt the fork-local code to the new upstream layout (or drop it if it's no longer needed), not work around the symptom.

## Recognizing this pattern

Signals that route here:

- `workflow` is `CI` (or the post-rebase `make check` step of `Fork preview`).
- `failed_step` is one of `Run go vet`, `Run tests`, `Run golangci-lint`, `Run make check`, or similar.
- Annotations point at real source files (not `.github`), with messages like `undefined: <name>`, `<name> redeclared`, `cannot find package`, `<name> undeclared`, or a lint rule name.
- The affected files are usually in directories upstream recently changed. `git log upstream/master -- <path>` often shows a relevant restructure.

## Fix recipe

1. Rebase locally to reproduce the failure:

```bash
git fetch upstream
git switch time-tracking
git rebase upstream/master
```

Expect the rebase to succeed. If it fails with a conflict, you're in the wrong pattern — switch to `rebase-merge-conflict.md`.

2. Confirm the CI failure reproduces locally:

```bash
make check
```

The error from `make check` should match what CI reported. If it doesn't, your diagnosis is wrong — stop and re-investigate before changing code. Divergence usually means local state is ahead of or behind origin in a way you didn't notice.

3. Figure out what upstream did to the affected area. Useful commands:

```bash
# What commits changed this path upstream since we last synced?
git log <our-base>..upstream/master -- <path>

# Where did the missing symbol go?
git grep -n '<symbol>' upstream/master -- '*.go'

# What files exist in the affected directory upstream now?
git ls-tree upstream/master <dir>
```

4. Port or remove the orphaned code. Two typical shapes:

   - Orphaned test file: test references helpers that moved to a new package. Move the test into the new package, update imports, rewrite calls against the new helper signatures. Delete the file from the old location.
   - Orphaned fork-local addition to a shared file: a helper/type/method the fork added is no longer called by anything (because the only caller was also removed or rewritten). Remove the fork-local addition so the file matches upstream — this reduces future rebase friction.

   The goal is minimum-divergence from upstream. Every fork-local change you can delete is one less conflict point next time upstream changes that file.

5. Re-run `make check` until green. If a second, unrelated failure surfaces as you fix the first, that's a sign there are multiple orphaned code paths — handle each as its own commit if they're logically distinct, or as one commit if they're all part of the same upstream restructure.

6. Return to SKILL.md step 5 (verify) and proceed through commit + push.

## Red flags (stop, don't auto-fix)

- The failure's not on a file the fork owns or modified — it's on upstream code. That's either a real upstream bug (not our problem to fix on the fork) or a dependency issue.
- The failure is a test assertion (`expected X, got Y`) rather than a compile error or lint rule. That's behavioral drift, not a structural port — needs human judgment about what the expected value should be now.
- The fix requires changing an upstream file (one the fork doesn't currently diverge from). Carrying a new local patch against upstream grows the fork's delta; prefer to open a discussion upstream instead.
