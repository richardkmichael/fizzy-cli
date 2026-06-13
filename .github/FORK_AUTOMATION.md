# Fork automation

How the `time-tracking` fork keeps itself current, and what it needs to work. The workflows carry
inline comments for local detail; this file covers the parts that span more than one of them.

## Workflows

| Workflow            | Trigger              | Does                                                       |
| ------------------- | -------------------- | ---------------------------------------------------------- |
| `fork-sync.yml`     | Schedule, dispatch   | Mirrors master from upstream, rebases `time-tracking`, pushes |
| `fork-preview.yml`  | Schedule, dispatch   | Dry-run rebase and check — no push, no release             |
| `fork-release.yml`  | Push to time-tracking | Tags and runs goreleaser                                   |

`fork-preview.yml` exists to surface a broken rebase without touching the branch. It needs no
credentials, so it keeps reporting even when the sync token is dead.

## What the fork rebases onto

Upstream's latest stable release, resolved from the releases API — GitHub's "latest" excludes
prereleases, so release candidates are skipped without parsing version strings. The fork carries
only its own functionality and replays it onto that tag, so it releases when upstream does rather
than on every upstream commit.

The `upstream-base` tag on origin records the release the fork currently sits on, and the rebase
runs `--onto` against it. That indirection is load-bearing. `git rebase <tag>` does nothing when the
tag is already reachable from `HEAD`, and it reports success while doing so — a fork sitting on a
commit newer than the release being targeted would silently stay there. Naming the previous base
makes the replayed range exactly the fork's own commits, in either direction.

`fork-sync.yml` advances the marker before pushing the branch. The branch push is what triggers
`fork-release.yml`, and the release version is derived from the marker, so moving the marker second
would race the release against a base the fork has already left — publishing a version named for the
previous release, its commit count inflated by the upstream commits between the two.

The two pushes are not atomic, so the ordering picks which way a failure between them breaks. A
marker ahead of the branch is harmless: upstream's commits between two releases are never ancestors
of the fork's HEAD, so the replayed range is the fork's own commits whether the marker names the old
release or the new one. A marker left behind is not — the next run's range would treat upstream's
commits as the fork's. Recover from either by re-running the sync with `force`.

## SYNC_TOKEN

A personal access token, stored as a secret in the `Sync` environment, read only by `fork-sync.yml`.

Required permissions: repository metadata read, contents read/write, workflows read/write. Scope it
to this repository alone. Nothing needs access to the environment itself.

The workflows permission is not optional. Both branches take workflow-file writes on the normal
path: upstream changes its own workflows and those land when master is mirrored, and the fork's three
`fork-*.yml` files are replayed onto every rebase. `GITHUB_TOKEN` cannot create or update anything
under `.github/workflows/` no matter what `permissions:` grants it.

A PAT is required for two independent reasons:

- `GITHUB_TOKEN` cannot write workflow files, per above. Master is mirrored through the server-side
  `merge-upstream` API rather than a local push, which keeps that write off the runner, but the call
  still authenticates as the PAT.
- Pushes authenticated with `GITHUB_TOKEN` do not trigger other workflows — GitHub blocks that to
  stop recursive runs. `fork-release.yml` fires on push to `time-tracking`, so the sync's force-push
  must be authored by the PAT or no release is ever cut. That is why the PAT is passed as the
  `actions/checkout` credential and not only to the sync step.

### Rotation

The token expires. Nothing warns when it does — the sync simply stops, and because releases are
downstream of the sync's push, releases stop with it. The failure is unmistakable once you look:
`fork-sync.yml` fails within seconds on its first step with `HTTP 401: Bad credentials` against
`merge-upstream`. Meanwhile `fork-preview.yml` keeps passing, since it needs no token.

To rotate: issue a replacement with the permissions above and update the `SYNC_TOKEN` secret in the
`Sync` environment. Then dispatch `fork-sync.yml` manually rather than waiting for the schedule.

## The Sync environment

`SYNC_TOKEN` is an environment secret, so only a job declaring `environment: Sync` can read it. This
mirrors how upstream scopes its signing keys to a `release` environment.

Treat this as defense-in-depth, not a security boundary. The environment has no protection rules and
no deployment branch policy, so anyone able to add a `${{ secrets.SYNC_TOKEN }}` reference to a
workflow could add the `environment:` line alongside it. What the scoping buys is that the token is
never ambient to jobs that do not ask for it, that its use appears in the environment's deployment
log, and that required reviewers or a branch policy can be attached later without moving the secret.

A fork that auto-syncs executes upstream's CI code against its own credentials. That exposure is
structural. Keeping the PAT limited to this repository, with a short expiry, is what bounds it.

## Two traps

Go toolchain ordering. `actions/setup-go` resolves `go-version-file` against the working tree and
pins `GOTOOLCHAIN=local`, so it must run after the rebase. Installed beforehand it resolves the
pre-rebase `go.mod`, and any upstream bump to the `go` directive then leaves every later Go step on a
toolchain older than the tree requires.

Skip-release markers. `fork-release.yml` skips when the head commit's subject opens with the marker
`[` `skip release` `]`. It must lead the subject — the push payload carries only `message`, since
git stores no separate subject, so the condition anchors at position 0 of that message. A marker
placed later in the subject, or anywhere in the body, has no effect. That is deliberate: an
unanchored match means prose describing the marker suppresses a release as surely as intending one.

Placement is anchored, but persistence is not. Fork commits are replayed onto every rebase, so a
marker keeps applying for as long as its commit remains the branch tip — not just to the push that
introduced it. Landing another fork commit on top clears it. Reserve the marker for a commit you
expect to build on, and don't assume a single use is spent after one push.
