# spm — shell-script package manager for mise + private storage

[![CI](https://github.com/kalw/spm/actions/workflows/ci.yml/badge.svg)](https://github.com/kalw/spm/actions/workflows/ci.yml)
[![Release](https://github.com/kalw/spm/actions/workflows/release.yml/badge.svg)](https://github.com/kalw/spm/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/go-1.27-00ADD8)
![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)

**Platforms** — cross-compiled in CI:

| OS | amd64 | arm64 |
|----|:-----:|:-----:|
| 🐧 linux | ✅ | ✅ |
| 🍎 macOS | ✅ | ✅ |
| 😈 FreeBSD | ✅ | ✅ |
| 🐡 OpenBSD | ✅ | ✅ |

`spm` packages shell scripts into **semver**-versioned, checksummed tarballs and publishes
them to **private storage** (HTTP, S3, or GCS). Consumers install them with
[mise](https://mise.jdx.dev) using its **native** `http:` / `s3:` backends — there is no
plugin or runtime component to install on the consumer side.

## Why it's simple

mise already knows how to download, version-resolve, checksum, and PATH-expose tools from a
URL or an S3 bucket. `spm` only does the two things mise doesn't:

1. **Package** a bundle of scripts into a reproducible `name-version.tar.gz` (+ `.sha256`).
2. **Publish** it to your private store and maintain a semver-sorted `versions.json`.

Then `spm add` prints the exact mise `[tools]` snippet for your storage type.

## Install

**With mise** (recommended) — picks the right binary for your OS/arch from
GitHub Releases and puts `spm` on your `PATH`:

```bash
mise use -g github:kalw/spm@0.1.2
```

**Prebuilt binary** (pick your OS/arch from the table above):

```bash
VERSION=v0.1.2
OS=$(uname -s | tr '[:upper:]' '[:lower:]')   # linux, darwin, freebsd, openbsd
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/kalw/spm/releases/download/${VERSION}/spm-${OS}-${ARCH}.tar.gz" \
  | tar -xz && sudo install spm /usr/local/bin/spm
spm version
```

**From source** (Go 1.27+):

```bash
go install github.com/kalw/spm/cmd/spm@latest
# or, in a checkout:
go build -o spm ./cmd/spm
```

## Package layout

A package is a directory with a manifest and its scripts:

```
deploy-tools/
  spm.toml
  src/deploy.sh
  src/rollback.sh
```

```toml
# spm.toml
name = "deploy-tools"      # [a-z0-9-]; becomes the mise tool name
version = "1.4.2"          # strict semver, validated at package/publish time
description = "Deploy helpers"

[[bin]]
name = "deploy"            # exposed command; lands at bin/deploy in the tarball
path = "src/deploy.sh"

[[bin]]
name = "rollback"
path = "src/rollback.sh"
```

Each `[[bin]]` is copied to `bin/<name>` (mode 0755) inside the tarball, so mise's backends
auto-expose them on `PATH`. Builds are reproducible (sorted entries, zeroed mtimes) — the same
inputs always yield the same sha256.

## Configure remotes

Remotes live in `~/.config/spm/config.toml` (override with `$SPM_CONFIG`). **Secrets are never
stored there** — they come from the environment / cloud SDK credential chains. See
[`examples/config.example.toml`](examples/config.example.toml).

| Type | Publish auth | Consumer backend |
|------|--------------|------------------|
| `s3`  | AWS credential chain (`AWS_*`, profile, IAM) | `s3:` |
| `gcs` (`access = "interop"`) | `SPM_GCS_HMAC_KEY` / `SPM_GCS_HMAC_SECRET` | `s3:` (GCS interop endpoint) |
| `gcs` (`access = "native"`) | ADC / `GOOGLE_APPLICATION_CREDENTIALS` | `http:` (public/signed `public_base_url`) |
| `http` (`publish = "webdav"`) | `SPM_HTTP_USER`/`SPM_HTTP_PASS` or `SPM_HTTP_TOKEN` | `http:` |
| `http` (`publish = "local"`)  | writes to `publish_dir` (served by nginx/etc.) | `http:` |

## Workflow

```bash
spm init ./deploy-tools                 # scaffold spm.toml + src/
spm package ./deploy-tools              # -> dist/deploy-tools-1.4.2.tar.gz + .sha256
spm publish ./deploy-tools --to prod-s3 # upload + update versions.json (semver-sorted)
spm list --from prod-s3 deploy-tools    # 1.0.0 / 1.2.0 / 1.4.2
spm add deploy-tools --from prod-s3     # print the mise.toml snippet (add --write to append)
```

Published versions are **immutable**: `publish` refuses to overwrite an existing version
unless you pass `--force`.

## Storage layout

```
<prefix>/<name>/versions.json                  # ["1.0.0","1.2.0","1.4.2"]
<prefix>/<name>/<name>-<version>.tar.gz
<prefix>/<name>/<name>-<version>.tar.gz.sha256
```

## Consuming with mise

`spm add` emits a native mise block. For an `http` remote:

```toml
[tools."http:deploy-tools"]
version = "1.4.2"
url = "https://dl.acme.com/spm/deploy-tools/deploy-tools-{{version}}.tar.gz"
checksum_url = "https://dl.acme.com/spm/deploy-tools/deploy-tools-{{version}}.tar.gz.sha256"
version_list_url = "https://dl.acme.com/spm/deploy-tools/versions.json"
version_order = "semver"
```

For `s3` (and GCS interop, which also sets `endpoint = "https://storage.googleapis.com"`):

```toml
[tools."s3:deploy-tools"]
version = "1.4.2"
url = "s3://acme-artifacts/spm/deploy-tools/deploy-tools-{{version}}.tar.gz"
version_list_url = "s3://acme-artifacts/spm/deploy-tools/versions.json"
version_order = "semver"
region = "us-east-1"
```

Then:

```bash
mise install           # downloads + verifies checksum, exposes deploy/rollback on PATH
mise use http:deploy-tools@1.4.2
```

Because `version_order = "semver"`, consumers can pin an exact version or track `latest`.

## Development

```bash
make build          # build ./spm
make check          # go vet + unit tests
make cross          # cross-compile every platform into dist/
make dist           # cross-compile + tar.gz + SHA256SUMS
make help           # list all targets
```

### Integration tests

The S3 adapter (which also powers the GCS S3-interop path) is covered by
MinIO-backed integration tests, guarded by the `integration` build tag so the
default `go test` stays fast and offline. With Docker available:

```bash
make integration    # starts MinIO, runs the tagged tests, tears MinIO down
```

Or point the tests at any existing S3-compatible endpoint:

```bash
AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
SPM_IT_S3_ENDPOINT=https://minio.example.com SPM_IT_S3_BUCKET=spm-it \
go test -tags integration -count=1 ./internal/storage/...
```

They are skipped automatically when `SPM_IT_S3_ENDPOINT` is unset.
