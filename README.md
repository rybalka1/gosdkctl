# gosdkctl

[Русская версия](README.ru.md)

`gosdkctl` is a rootless Go SDK manager. It keeps Go versions in `~/sdk`, tracks the default SDK through `~/sdk/go-current`, and never writes to `/usr/local` or other root-owned paths.

The workflow is intentionally similar to tools like `uv`: one small binary owns installation, discovery, version switching, shell integration, and diagnostics for a local developer environment.

## Supported Platforms

Release binaries are currently published for:

```text
gosdkctl-linux-amd64
gosdkctl-darwin-arm64
```

On both Linux and macOS, `gosdkctl install latest` downloads the Go SDK archive for the current `GOOS/GOARCH`, verifies the official SHA256 checksum, and installs it under `~/sdk`.

## Quick Start: Linux amd64

On a clean Linux amd64 machine, you do not need to install Go manually. Download the release binary first:

```bash
curl -L -o gosdkctl https://github.com/rybalka1/gosdkctl/releases/download/v1.0.0/gosdkctl-linux-amd64
chmod +x gosdkctl
./gosdkctl setup zsh
~/.local/bin/gosdkctl install latest
exec zsh
```

## Quick Start: macOS arm64

On Apple Silicon macOS, use the darwin arm64 release binary. The flow is otherwise the same as Linux:

```bash
curl -L -o gosdkctl https://github.com/rybalka1/gosdkctl/releases/download/v1.0.0/gosdkctl-darwin-arm64
chmod +x gosdkctl
./gosdkctl setup zsh
~/.local/bin/gosdkctl install latest
exec zsh
```

For bash on either platform, run `setup bash` instead of `setup zsh`:

```bash
./gosdkctl setup bash
~/.local/bin/gosdkctl install latest
exec bash
```

Then verify the environment:

```bash
gosdkctl current
go version
```

## Directory Layout

```text
~/sdk/
  go1.24.2/
  go1.25.1/
  go1.26.0/
  go-current -> $HOME/sdk/go1.26.0
```

## Commands

```text
gosdkctl status
gosdkctl list
gosdkctl current
gosdkctl install <archive.tar.gz|goX.Y.Z|latest>
gosdkctl migrate-local
gosdkctl setup [zsh|bash|auto]
gosdkctl init [zsh|bash|auto]
gosdkctl self install
gosdkctl switch <goX.Y.Z>
gosdkctl switch
gosdkctl choose
gosdkctl doctor
gosdkctl env [goX.Y.Z|path|default]
```

`switch` without an argument behaves like `choose` and prompts for one of the installed versions.

## Install a Go SDK

Install the latest stable Go release:

```bash
gosdkctl install latest
```

Install a specific version:

```bash
gosdkctl install go1.26.1
```

Install a local archive:

```bash
gosdkctl install ~/Downloads/go1.26.1.linux-amd64.tar.gz
gosdkctl install ~/Downloads/go1.26.1.darwin-arm64.tar.gz
```

When downloading from `go.dev`, `gosdkctl` selects the archive for the current `GOOS/GOARCH` and verifies its SHA256 checksum from the official download metadata. It then extracts the SDK into `~/sdk/go1.26.1`, validates `go/VERSION` and `go/bin/go`, and updates `~/sdk/go-current`. Existing SDK directories are kept. Reinstalling the same version is idempotent: the existing SDK is reused and becomes the default.

## Migrate Legacy `~/.local/go`

If an older Go installation lives in `~/.local/go`, migrate it explicitly:

```bash
gosdkctl migrate-local
```

The command reads `~/.local/go/VERSION`, moves the directory into `~/sdk/<version>`, and updates `~/sdk/go-current`. If that version already exists in `~/sdk`, it is not overwritten; the existing SDK becomes the default.

## Switch the Default SDK

```bash
gosdkctl switch go1.24.2
gosdkctl current
```

Only `~/sdk/go-current` is changed. Already-open shell sessions need their environment refreshed.

## Shell Integration

Install the managed block into your shell config:

```bash
gosdkctl init zsh
```

For bash:

```bash
gosdkctl init bash
```

The command rewrites only the block between `# >>> gosdkctl init >>>` and `# <<< gosdkctl init <<<` in `~/.zshrc` or `~/.bashrc`. The rest of the user config is preserved.

New shell sessions get `go`, `gosdkctl`, `go-sdk`, `usego`, `gosetdefault`, and `gocurrent`.

The managed block resolves `GOROOT` with this fallback chain: `~/sdk/go-current`, legacy `~/.local/go`, then the newest `goX.Y.Z` directory in `~/sdk`.

A binary cannot directly mutate the already-running parent shell, so `gosdkctl env` can also print shell exports:

```bash
eval "$(gosdkctl env go1.24.2)"
eval "$(gosdkctl env default)"
eval "$(gosdkctl env /opt/custom-go)"
```

The managed block adds these helpers:

```bash
usego() {
  eval "$(gosdkctl env "${1:-default}")"
}

gosetdefault() {
  gosdkctl switch "$1" || return
  usego default
}

gocurrent() {
  gosdkctl current
  which go
  go version
}
```

## Diagnostics

```bash
gosdkctl doctor
```

The report includes `GOROOT`, `GOPATH`, `PATH`, the `go-current` target, whether legacy `~/.local/go` exists, the `go` binary visible through `PATH`, and installed SDK versions.

## Build From Source

Building from source is only needed when developing `gosdkctl` itself. For bootstrapping a clean machine, use the release binary from GitHub Releases.

```bash
go build -o ~/.local/bin/gosdkctl ./cmd/gosdkctl
```

## Self Install

If the binary is running from a temporary location, it can install itself into the standard user path:

```bash
gosdkctl self install
```

The command creates `~/.local/bin/gosdkctl` and a compatible symlink at `~/.local/bin/go-sdk`.

## Clean Machine Bootstrap

Minimal flow after downloading the platform-specific release binary:

```bash
./gosdkctl setup zsh
~/.local/bin/gosdkctl install latest
exec zsh
```

For bash:

```bash
./gosdkctl setup bash
~/.local/bin/gosdkctl install latest
exec bash
```

## License

MIT License. See [LICENSE](LICENSE).
