# Workflow tool not installed

A `make check` (or individual make target) step in a fork workflow failed because a required tool — most commonly `golangci-lint` — is not installed on the runner. This is a workflow configuration gap, not a code problem.

## Recognizing this pattern

Signals that route here:

- `failed_step` is `Verify rebased branch compiles and passes tests` or similar.
- `log_tail` contains `make: <tool>: No such file or directory` or `<tool>: command not found`.
- The error is in a fork workflow (`Fork sync`, `Fork preview`) rather than the upstream `CI` workflow.
- No annotations on source files — the failure is in the shell, not in compiled code.

Common tools:
- `golangci-lint` — required by the `lint` target in `make check`

## Fix recipe

1. Identify which tool is missing from the log tail.

2. Check how `fork-preview.yml` installs the same tool — it is the reference workflow that already handles this correctly. For `golangci-lint`:

```yaml
- name: Install golangci-lint
  uses: golangci/golangci-lint-action@1e7e51e771db61008b38414a730f564565cf7c20 # v9.2.0
  with:
    version: v2.10
    args: --help
```

3. Add the same install step to the failing workflow, immediately before the `make check` step. Keep the pinned SHA identical to `fork-preview.yml` to stay in sync.

4. Verify locally that `make check` passes (the tool is already installed on your machine — this just confirms the fix is correct before pushing).

5. Return to SKILL.md step 6 (commit + push). No force-push needed if `time-tracking` hasn't been rebased — a regular push is fine.

## Red flags (stop, don't auto-fix)

- The missing tool is something other than `golangci-lint` and you don't have a reference install step. Look it up before adding an unpinned action.
- The tool is installed but a version mismatch causes a failure — that's a different problem (version drift between workflows), not a missing install.
