# Retail preparation

Prepare the existing deployment's baseline inputs from an installed game. This is an
explicit maintenance step, separate from building, checking and deploying.

## Prepare or verify

```powershell
# Prepare both games using the same Windows registry discovery as the checksum CLI.
./scripts/prepare-retail-snapshots.ps1

# Prepare one game or override its detected installation.
./scripts/prepare-retail-snapshots.ps1 -Game generalsmd -ZeroHourRoot 'D:\Games\Zero Hour'

# Verify existing generated files without consulting the registry or copying anything.
./scripts/prepare-retail-snapshots.ps1 -Verify
```

`-GeneralsRoot` and `-ZeroHourRoot` take priority over registry lookup for their respective
games. `-Output` changes the destination; it defaults to `worker/retail` in the repository.
Explicit relative installation and output paths are resolved from your current directory.
When one game is selected, preparation leaves the other game's directory unchanged.
Both games must be prepared before deployment because the service supports both.

The underlying Go command is usable without PowerShell, including on Linux:

```sh
go run ./cmd/gencrc-snapshots \
  --generals-root '/games/Generals' \
  --zero-hour-root '/games/Zero Hour' \
  --output worker/retail

go run ./cmd/gencrc-snapshots --verify --output worker/retail
```

Run those Go examples from the repository root. On non-Windows platforms supply explicit
roots; no Steam library location is hardcoded or guessed.

## What is prepared

| Game | INI archive | Script files |
| --- | --- | --- |
| Generals 1.08 (`generals`) | `INI.big` | `Data/Scripts/SkirmishScripts.scb`, `MultiplayerScripts.scb` |
| Zero Hour 1.04 (`generalsmd`) | `INIZH.big` | `Data/Scripts/SkirmishScripts.scb`, `MultiplayerScripts.scb` |

Script files are copied from `Data/Scripts/` and stored beside the archive in each
prepared game directory. Each output directory contains a deterministic manifest with
file sizes and SHA-256 hashes. No executables are copied.

These are **copies of retail files**, not intermediate checksum-state snapshots.
The INI engine needs the file contents to apply Additional and Mod Files; the current
EXE CRC calculation reads script bytes together with executable-dependent state.

## Integrity and replacement

The checked-in baseline lock contains metadata only. Its initial fingerprints pin the
bytes used by deployment `baee4072-6612-460f-9f11-5fbb0ab1a10d` on 2026-09-11. This makes
preparation reproducible; it is not independent proof of publisher authenticity or an
allowlist of every legitimate retail distribution.

Registry discovery identifies a directory, not an unmodified installation. Preparation
rejects missing or changed files, even if the registry path is valid. Use an installation
matching the supported baseline. Do not automatically rewrite the lock to accept a
modified installation. Supporting another baseline requires deliberate hash/provenance
review and checksum regression tests.

Preparation validates all selected inputs before replacing existing output. New data is
staged and verified before installation, with rollback on replacement errors. It never
writes to the source game directory. Keep the destination separate from game roots;
unsafe overlapping paths are rejected.

An interrupted operation or failed rollback leaves a `.gencrc-prepare.lock` directory
with recovery data. Preparation and verification then stop rather than trusting a
partially replaced baseline. Preserve that directory, inspect the reported backup paths,
and restore the prior snapshots before clearing the lock; do not blindly delete it.

## Deployment boundary

Retail files are private deployment inputs and must remain out of Git and public build
artifacts. Wrangler runs the Worker before static-asset routing; only the logo/favicon
routes may expose an asset. INI archives, scripts and manifests are fetched internally
by the checksum implementation, not exposed as download endpoints.

Changing asset routing or introducing a catch-all assets handler can break this boundary.
The runtime regression test uses synthetic assets to check that public retail paths
remain inaccessible while internal baseline calculation still works.
