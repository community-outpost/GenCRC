# GenCRC

Calculate Generals and Zero Hour EXE and INI checksums from a CLI or the **Gen//CRC** web app.
The checksum engine is shared Go code; the Cloudflare Worker runs it as WebAssembly.

- [CLI reference](docs/cli.md) — file inputs, install discovery and archive precedence.
- [API reference](docs/api.md) — multipart uploads, responses and examples.
- [Retail preparation](docs/retail.md) — baseline fingerprints and safe regeneration.

## Build and check

Prerequisites: Go matching `go.mod`; Node.js 22 or newer and npm for Worker builds;
Bun 1.3.1 for tests. Run scripts with PowerShell 7 (`pwsh`) on any supported OS, or
Windows PowerShell 5.1 on Windows. The CLI itself uses only the Go standard library.
The npm shortcuts invoke `pwsh`; use the scripts directly if you only have Windows PowerShell.

```powershell
npm ci --prefix worker
./scripts/build.ps1                 # CLI + Worker; no retail preparation or deployment
./scripts/check.ps1                 # Go tests/vet + WASM + JavaScript/runtime tests
```

Build just one target with `-Target cli` or `-Target worker`. CLI-only builds need Go,
not Node or Bun. Build paths are relative to the repository, not your current directory.

| Output | Location |
| --- | --- |
| Native CLI (`gencrc.exe` on Windows, `gencrc` elsewhere) | `dist/cli/` |
| Worker bundle from Wrangler's dry run | `dist/worker/` |
| WASM and Go's JavaScript runtime shim | `worker/generated/` |

Checks and builds do not require game files. Tests generate their own tiny fixtures.
CI runs the same commands on Windows and Linux; it does not deploy or upload retail data.

## Use the CLI

```powershell
# INI only; discover the Zero Hour installation from the Windows registry.
./dist/cli/gencrc.exe --game generalsmd --sideload patch.big

# EXE only, using the installed script files.
./dist/cli/gencrc.exe --game generalsmd --exe Game.dat
```

The requested checksums are inferred from file inputs; there is no mode flag.
Use `--installpath` to override discovery or run against an installation on another OS.
See the [CLI reference](docs/cli.md) for combined inputs and mod behavior.

## Run the website locally

Retail snapshots are required for real checksum requests, not for building the website.
Prepare them explicitly from your own installations:

```powershell
./scripts/prepare-retail-snapshots.ps1  # Registry discovery on Windows; both games
npm --prefix worker run dev
```

Preparation accepts explicit installation roots and verifies the pinned baseline before
replacing existing output. See [retail preparation](docs/retail.md) for single-game and
non-Windows use. The website keeps raw retail inputs behind the Worker; they are not
public download routes.

## Deploy

```powershell
./scripts/check.ps1
npm --prefix worker run deploy
```

Deploy is explicit. It verifies both prepared baselines and builds fresh WASM before
calling Wrangler. Configure your Cloudflare credentials and account in your environment;
no account credentials belong in source. Do not publish `worker/retail/` through another
static host or upload generated retail data as CI artifacts.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `cmd/gencrc/` | Native checksum CLI |
| `cmd/gencrc-wasm/` | JavaScript/WASM boundary |
| `cmd/gencrc-snapshots/` | Retail preparation and verification CLI |
| `internal/` | Checksums, version detection, install discovery and snapshot validation |
| `worker/src/`, `worker/test/` | HTTP API, browser UI and tests |
| `worker/retail/` | Public logo plus ignored, internally accessed retail inputs |
| `scripts/` | Build/check entrypoints and preparation wrapper |
| `docs/` | CLI, API and baseline reference |

Generated output, caches and local Wrangler state are ignored. The scripts named
`build-worker.ps1` and `build:wasm` remain WASM-only compatibility entrypoints.
