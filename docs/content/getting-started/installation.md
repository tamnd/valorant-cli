---
title: "Installation"
description: "Install valorant from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/valorant-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `valorant` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/valorant-cli/cmd/valorant@latest
```

That puts `valorant` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/valorant-cli
cd valorant-cli
make build        # produces ./bin/valorant
./bin/valorant version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/valorant:latest --help
```

## Checking the install

```bash
valorant version
```

prints the version and exits.
