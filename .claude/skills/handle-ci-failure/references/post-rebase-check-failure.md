# Post-rebase check failure

The rebase completed cleanly, but the resulting tree fails the project's check command. This usually means upstream restructured code the project depends on — helpers moved, packages renamed, types removed — and the replayed commits still reference the old identities. The fix is always: adapt the local code to the new upstream layout, not work around the symptom.

## Recognizing this pattern

Signals that route here:

- `failed_step` is a check, test, vet, or lint step (e.g. `Run go vet`, `Run tests`, `make check`).
- Annotations point at real source files (not `.github`), with messages like `undefined: <name>`, `cannot find package`, `<name> undeclared`, or a lint rule name.
- The affected files are usually in directories upstream recently changed.

## Fix recipe

1. Rebase locally to reproduce the failure (branch and upstream base are in `CI*.md`):

```bash
git fetch upstream
git switch <branch>
git rebase <upstream-base>
```

Expect the rebase to succeed. If it fails with a conflict, switch to `rebase-merge-conflict.md`.

2. Confirm the CI failure reproduces locally:

```bash
<check command from CI*.md>
```

If the local error doesn't match CI, your diagnosis is wrong — stop and re-investigate before changing code.

3. Figure out what upstream did to the affected area:

```bash
# What commits changed this path upstream since we last synced?
git log <our-base>..<upstream-base> -- <path>

# Where did the missing symbol go?
git grep -n '<symbol>' <upstream-base> -- '*.go'

# What does the affected directory look like upstream now?
git ls-tree <upstream-base> <dir>
```

4. Port or remove the orphaned code:

- Orphaned call to a removed helper: replace with whatever upstream now exposes for the same purpose. Check the upstream commit that removed it — it often shows what replaced it.
- Orphaned addition to a shared file: if the only caller was also removed upstream, delete the addition. Minimum-divergence from upstream reduces future conflict surface.

5. Re-run the check command until green. If a second unrelated failure surfaces, handle each as its own commit if logically distinct, or as one commit if they're all part of the same upstream restructure.

6. Return to SKILL.md step 5 (verify) and proceed through commit + push.

## Red flags (stop, don't auto-fix)

- The failure is on upstream code, not on files the project owns or modified. That's an upstream bug — not our problem to fix on a fork.
- The failure is a test assertion (`expected X, got Y`) rather than a compile error or lint rule. That's behavioral drift, needs human judgment.
- The fix requires changing an upstream file the project doesn't currently diverge from. Carrying a new local patch grows the delta; prefer to open a discussion upstream instead.
