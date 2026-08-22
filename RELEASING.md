# Releasing the dkarter fork

Pushing a `v*` tag starts the fork-owned release workflow. It tests the tagged commit, builds all supported archives, includes and publishes the license, generates SBOMs, verifies checksums and archive contents, records GitHub build-provenance attestations, and then publishes the release to `dkarter/ghtkn`. It uses only the repository `GITHUB_TOKEN`.

Normal tags such as `v1.2.3` create stable releases. Tags with a prerelease suffix, such as `v1.2.3-rc.1`, create prereleases.

## Create a release tag

Do not push an upstream tag object directly: that commit does not contain this fork's release workflow and fork-specific fixes. First integrate the desired upstream tag into `main` through the normal review process. Then run the tests and create a fork-owned annotated tag at the reviewed fork commit:

```sh
version=v1.2.3
git switch main
git fetch origin main
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
test -z "$(git status --porcelain)"
go test ./...
git tag -a "$version" -m "ghtkn $version" "$(git rev-parse HEAD)"
git push origin "refs/tags/$version:refs/tags/$version"
```

The explicit refspec does not force-update an existing remote tag. If the local tag or remote tag already exists at a different object, stop and investigate instead of deleting or overwriting it. Do not create a test tag: validate packaging locally with the commands below.

## Validate without publishing

Install the versions of GoReleaser and Syft pinned in `.github/workflows/release.yaml`, then run:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Confirm `dist/` contains the six mise-compatible archives, `ghtkn_checksums.txt`, and an `.sbom.json` file for every archive. Each archive must contain `LICENSE`; the workflow also publishes it as a checksummed and attested standalone asset. No GitHub Release is created by the snapshot command.

## Repository setup

GitHub Actions must be enabled for the repository. The workflow requests only the release and provenance permissions it needs: `contents: write`, `id-token: write`, `attestations: write`, and `artifact-metadata: write`. No upstream bot credentials or `TAKUMI_*` secrets are used.
