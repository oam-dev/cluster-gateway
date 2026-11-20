# Releasing cluster-gateway

KubeVela ships cluster-gateway twice: as the gateway image in its `vela-core` chart, and as a Go module in its
`go.mod`. Each KubeVela line uses one cluster-gateway release line for both, so a fix reaches KubeVela users through a
cluster-gateway patch release and then a KubeVela patch release.

## Branches

| Branch | What it is | Tags | Used by KubeVela |
| --- | --- | --- | --- |
| `master` | development; the next minor release | none | — |
| `release-1.10` | the 1.10 line | `v1.10.x` (`v1.10.0-alpha.1` today) | `master`, `release-1.12` |
| `release-1.9` | the 1.9 line | `v1.9.x` (`v1.9.2` today) | `release-1.10`, `release-1.11` |

The release branches get security and bug fixes; new features go to `master`. A new minor line starts with a
`release-x.y` branch cut from `master` at the commit of its first tag.

## Backports

A fix goes to `master` first, then to the release branches that need it. The
[Backport workflow](.github/workflows/backport.yml) opens the backport pull requests:

- **Before merging**, add a label per branch: `backport release-1.9`, `backport release-1.10`. When the pull request is
  merged, the workflow cherry-picks its commits (with `-x`) onto each branch and opens one pull request per branch.
- **After merging**, a maintainer comments `/backport release-1.9` (or `/backport 1.9`; several branches separated by
  spaces) on the merged pull request. Adding a label after the merge does nothing.

If a commit doesn't apply, the workflow says so in a comment and the run fails; backport that branch by hand
(`git cherry-pick -x`) in a pull request titled `[Backport release-x.y] <title>`.

[Dependabot](.github/dependabot.yml) opens one grouped pull request a week per branch for the actions, and one for the
base images (new digests and patch versions only).

## What the workflows publish

[BuildImage](.github/workflows/build-image.yml) publishes the gateway to `docker.io/oamdev/cluster-gateway` and
`ghcr.io/oam-dev/cluster-gateway` (`linux/amd64` + `linux/arm64`):

| Trigger | Image tags |
| --- | --- |
| a push to `master` or `release-x.y` (or a manual run on one) | `<branch>` and `<branch>-<short sha>`: a candidate, not for users |
| a `vX.Y.Z` tag | `vX.Y.Z`; `latest` moves only if it is the highest final release (not an older line's patch or a pre-release) |

[GoRelease](.github/workflows/go-release.yml), on a `v*` tag:

- publishes `docker.io/oamdev/cluster-gateway-addon-manager:vX.Y.Z` (`linux/amd64` + `linux/arm64`);
- creates the GitHub Release with the `cluster-gateway` and `addon-manager` charts at version `X.Y.Z`. A tag with a
  `-` (`-alpha.1`, `-rc.1`) is a pre-release; only the highest final release is marked Latest.

[CI](.github/workflows/ci.yaml) runs the unit tests and the e2e tests on every push to `master` and `release-*`, and on
pull requests to them unless they change only docs (Markdown, images).

## Making a release

1. Merge the fixes into the release branch (backports above).
2. Take the candidate the merge published: `oamdev/cluster-gateway:release-x.y-<short sha>` (Actions → BuildImage →
   Run workflow on the branch rebuilds it).
3. Test the candidate: scan it, and run it under the KubeVela lines that use this branch (the `vela-core` chart with
   the candidate image, joining a cluster and reaching it through the gateway).
4. Tag the tested commit, `vX.Y.Z` on `release-x.y`. Tags are never moved or reused: a fix after tagging is the next
   patch.
5. Check the published images (they are rebuilt from the tag, so their digests differ from the candidate's) and the
   charts on the GitHub Release, then add the release notes to it: what changed, the image digests, and how to
   upgrade.
6. Update the KubeVela branches that use the line: the `vela-core` chart's gateway image (`tag` with the digest) and
   `go.mod`, both to the new tag. Then update the table above and in the [README](README.md).
