# Releasing the dkarter fork

Release Please manages releases from conventional commits merged into `main`. It opens or updates a release pull request containing the next version. Merging that pull request creates a `v*` tag and draft GitHub Release, then the same workflow tests the tagged commit, builds and validates all supported archives, uploads checksums, SBOMs, license material, and provenance, and publishes the completed release.

Do not create release tags manually. In particular, do not push an upstream tag object: that commit does not contain this fork's release automation or fork-specific fixes. Integrate upstream changes into `main` through the normal review process and let Release Please select the next fork version.

Release Please creates the draft before the build so users cannot observe a partially populated published release. With release immutability enabled for the repository, publishing the draft permanently locks its tag and assets.

If a release build fails after the draft is created, rerun the failed jobs from that original workflow run. Asset uploads replace any partial draft assets before publication, so the retry does not require deleting the tag or release.

## Validate without publishing

Install the versions of GoReleaser and Syft pinned in `.github/workflows/release.yaml`, then run:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Confirm `dist/` contains the six mise-compatible archives, `ghtkn_checksums.txt`, and an `.sbom.json` file for every archive. Each archive must contain `LICENSE`; the workflow also publishes it as a checksummed and attested standalone asset. No GitHub Release is created by the snapshot command.

## Repository setup

GitHub Actions must be enabled for the repository. Create and install a GitHub App on `dkarter/ghtkn` with repository `Contents: Read and write` and `Pull requests: Read and write` permissions. Configure these Actions values:

- Repository variable `RELEASE_CLIENT_ID`: the GitHub App client ID.
- Repository secret `RELEASE_PRIVATE_KEY`: the GitHub App private key.

In the repository's **Settings > Releases**, enable **Release immutability**. This setting applies only to releases published after it is enabled.

The App token is used only to maintain the release pull request and create its tag and draft release. Asset upload, provenance, and final publication use the job-scoped `GITHUB_TOKEN`. No upstream bot credentials or `TAKUMI_*` secrets are used.
