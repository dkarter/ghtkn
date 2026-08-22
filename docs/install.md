---
description: Install the ghtkn CLI and verify downloaded release assets. Use when installing the maintained dkarter fork through mise or GitHub Releases, using upstream package managers, or verifying checksums and GitHub attestations.
---

# Install

ghtkn is written in Go. So you only have to install a binary in your `PATH`.

There are some ways to install ghtkn.

1. [mise](#mise)
1. [Homebrew](#homebrew)
1. [Scoop](#scoop)
1. [aqua](#aqua)
1. [GitHub Releases](#github-releases)
1. [Build an executable binary from source code yourself using Go](#build-an-executable-binary-from-source-code-yourself-using-go)

## mise

Install this maintained fork directly from its GitHub Releases:

```sh
mise use -g github:dkarter/ghtkn
```

The release archives use mise's expected names, such as `ghtkn_darwin_arm64.tar.gz` and `ghtkn_linux_amd64.tar.gz`.

## Homebrew

You can install ghtkn using [Homebrew](https://brew.sh/).

```sh
brew install suzuki-shunsuke/ghtkn/ghtkn --cask
```

## Scoop

You can install ghtkn using [Scoop](https://scoop.sh/).

```sh
scoop bucket add suzuki-shunsuke https://github.com/suzuki-shunsuke/scoop-bucket
scoop install ghtkn
```

## aqua

[aqua-registry >= v4.407.0 is required](https://github.com/aquaproj/aqua-registry/releases/tag/v4.407.0).

You can install ghtkn using [aqua](https://aquaproj.github.io/).

```sh
aqua g -i suzuki-shunsuke/ghtkn
```

## Build an executable binary from source code yourself using Go

```sh
go install github.com/suzuki-shunsuke/ghtkn/cmd/ghtkn@latest
```

## GitHub Releases

You can download an asset from [the dkarter fork's GitHub Releases](https://github.com/dkarter/ghtkn/releases).
Please unarchive it and install a pre built binary into `$PATH`. 

### Verify downloaded assets from GitHub Releases

Each fork release includes SHA-256 checksums, per-archive SPDX JSON SBOMs, and GitHub artifact attestations with build provenance. Verify an archive with the [GitHub CLI](https://cli.github.com/):

You can install GitHub CLI by aqua.

```sh
aqua g -i cli/cli
```

```sh
version=v0.1.0
asset=ghtkn_darwin_arm64.tar.gz
gh release download -R dkarter/ghtkn "$version" -p "$asset"
gh attestation verify "$asset" \
  -R dkarter/ghtkn \
  --signer-workflow dkarter/ghtkn/.github/workflows/release.yaml
gh release download -R dkarter/ghtkn "$version" -p ghtkn_checksums.txt
sha256sum --check --ignore-missing ghtkn_checksums.txt
```
