# API reference

The browser includes this contract under **API reference**. Use `GET /api` for machine-readable discovery.

## Endpoints

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/` | Browser checksum form. |
| `GET` | `/page-client.js` | Standalone browser JavaScript module. |
| `GET` | `/logo.png` or `/favicon.ico` | Public Community Outpost logo. |
| `GET` | `/api` | Machine-readable fields, manifest schemas, modes, and limits. |
| `GET` | `/api/retail?game=generalsmd` | Zero Hour 1.04 retail INI baseline; use `generals` for Generals 1.08. |
| `POST` | `/api/checksum` | Calculate checksums from multipart input. |
| `OPTIONS` | `/api/checksum` or `/api/retail` | CORS preflight (`204`, no body). |

Other method/path combinations return `404` JSON.
Raw retail archive, script and manifest paths are not public endpoints, including
`GET` and `HEAD` requests. They are accessible only through the Worker's internal asset binding.

## Retail baseline (no upload)

Select **Retail baseline** in the browser, then choose Zero Hour 1.04 or Generals 1.08.
Switch back to **Your files** to restore your selected uploads. Retail mode never sends
them. The API equivalent is:

```shell
curl 'https://your-worker.example/api/retail?game=generalsmd'
```

Returns `{ "game": "generalsmd", "version": "1.04", "ini_crc": "FEAAE3F3" }`.
Use `game=generals` for Generals 1.08 (`98FA8CC7`). Values are calculated from the bundled
retail INI archives with no Additional or Mod Files. No EXE CRC is provided: executable
bytes vary between distributions; upload an executable to calculate its checksum.
Missing or invalid `game` returns `422` / `game_required`.

## Multipart contract

Send `POST /api/checksum` as `multipart/form-data`.

| Field | Kind | Contract |
| --- | --- | --- |
| `game` | text | Required without `exe`; `generals` or `generalsmd`. Ignored with `exe`. |
| `exe` | one file | Optional. Supplies `exe_crc`, version metadata, and authoritative game detection. |
| `additional_manifest` | JSON text | Ordered array of Additional Files sources. |
| `additional_file` | repeated file | Blob list indexed by `additional_manifest`. |
| `mod_manifest` | JSON text | One optional Mod Files source. |
| `mod_file` | repeated file | Blob list indexed by `mod_manifest`. |

At least one non-empty `exe`, `additional_file`, or `mod_file` is required. Additional
Files are loaded as if added to the game folder. Each source has this shape:

```json
{
  "kind": "big",
  "files": [{ "name": "010_patch.big", "index": 0 }]
}
```

A `big` source has exactly one file. A folder source preserves virtual paths:

```json
{
  "kind": "folder",
  "name": "PatchFolder",
  "files": [
    { "name": "Data/INI/Weapon.ini", "index": 1 },
    { "name": "020_assets.big", "index": 2 }
  ]
}
```

`index` is zero-based within the corresponding repeated file field. The
`additional_manifest` array is source order. Contained BIGs are indexed in sorted path
order. Loose files from later folder sources override earlier loose roots and all
archives. Archive conflicts use the CLI's case-insensitive basename precedence: the
alphabetically first Additional Files archive wins.

Mod Files are supplied through the game's `-mod` option. `mod_manifest` is one source in
the same shape. A `big` source is one BIG. A `folder` source may list folder contents,
but only `.big` files participate; they are sorted by path and applied as overriding
archives, so the last sorted mod archive wins conflicts. Additional loose files retain
the CLI's loose-file precedence over mod archives.

This `-mod` boundary is verified against the game source: `CommandLine.cpp` parses one
path and assigns either `m_modDir` or `m_modBIG`; `ArchiveFileSystem.cpp::loadMods`
opens `m_modBIG` as one archive, while `m_modDir` calls
`loadBigFilesFromDirectory(modDir, "*.big", TRUE)`. A mod directory therefore does not
load loose INI files.

The service supports EXE-only, Additional-Files-only, Mod-Files-only, and any combined
mode. `ini_crc` is returned whenever Additional or Mod Files participate. When `exe` is
present, Go detects the game before retail snapshot selection; a conflicting submitted
`game` never controls the result.

The complete request body must not exceed 64 MiB when `Content-Length` is present. The
sum of decoded EXE, Additional Files, and Mod Files must also not exceed 64 MiB. Empty
file fields are absent. Paths must be relative and must not contain `..` segments.

## Success responses

Every response contains `game`. CRC values are uppercase eight-digit hexadecimal.
EXE-only responses add `exe_crc` and `version`; if recognized, they also add
`generalsonline_version`. Any INI-input mode adds `ini_crc`. The uploaded filename is
never returned.

```json
{
  "exe_crc": "A7471E47",
  "game": "generalsmd",
  "generalsonline_version": "082826_QFE1",
  "ini_crc": "FEAAE3F3",
  "version": "1.04"
}
```

## Errors

Errors use `{ "error": { "code": "...", "message": "..." } }`.

| Status | Code | Cause |
| --- | --- | --- |
| `400` | `missing_input` | No non-empty EXE, Additional Files, or Mod Files. |
| `400` | `invalid_manifest` | Invalid JSON/schema/path/index, missing manifest for files, or invalid BIG source. |
| `404` | `not_found` | Method/path is not defined. |
| `413` | `upload_too_large` | Request body or decoded file sum exceeds 64 MiB. |
| `422` | `game_required` | Game is invalid or missing when no executable is uploaded. |
| `422` | `launcher_executable` | Recognized game launcher; upload the installation's `Game.dat` instead. |
| `422` | `unsupported_executable` | Game identity or version could not be recognized. This does not necessarily mean the file is a launcher. |
| `422` | `checksum_failed` | Multipart, snapshot, Wasm, BIG, or checksum processing fails. |

All JSON responses include permissive CORS (`Allow-Origin: *`, methods
`GET, POST, OPTIONS`, header `content-type`, max age `86400`) and `Cache-Control:
no-store`. The HTML response does not add CORS headers.

## Discovery and curl

```shell
curl https://your-worker.example/api

# EXE only
curl -X POST https://your-worker.example/api/checksum -F exe=@Game.dat

# One standalone Additional Files BIG
curl -X POST https://your-worker.example/api/checksum \
  -F game=generalsmd \
  -F 'additional_manifest=[{"kind":"big","files":[{"name":"010_patch.big","index":0}]}]' \
  -F additional_file=@010_patch.big

# Mod BIG combined with an EXE
curl -X POST https://your-worker.example/api/checksum \
  -F exe=@Game.dat \
  -F 'mod_manifest={"kind":"big","files":[{"name":"MyMod.big","index":0}]}' \
  -F mod_file=@MyMod.big
```

