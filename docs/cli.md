# CLI reference

Calculate legacy `exeCRC` and `iniCRC` values.

```shell
go run ./cmd/gencrc [--game generals|generalsmd] [--installpath PATH]
                   [--exe NAME|ABSOLUTE_PATH]
                   [--sideload PATH ...] [--mod PATH] [-v]
```

Build with `./scripts/build.ps1 -Target cli`; outputs are in `dist/cli/`.

No dependencies beyond the Go standard library.

The tool detects the game from the selected executable, then falls back to `INIZH.big` or
`INI.big`.

```shell
# INI CRC only; discover the Zero Hour installation on Windows.
go run ./cmd/gencrc --game generalsmd --sideload patch.big

# EXE CRC and version only; no INI files are needed.
go run ./cmd/gencrc --game generalsmd --exe Game.dat

# Both CRCs from an explicitly selected installation.
go run ./cmd/gencrc --installpath PATH --exe Game.dat --mod MyMod.big
```

The requested CRCs are inferred from your file inputs; no mode flag is needed:

| Inputs | Result |
| --- | --- |
| `--exe` only | EXE CRC and version |
| `--sideload`, `--mod`, or `--fallback-generals-root`, without `--exe` | INI CRC |
| `--exe` plus any of those INI inputs | Both CRCs and version |
| No file inputs | Both CRCs for the selected installation |

INI-only requests do not require or read an executable.
EXE-only requests do not load INI files, but require the installation's
`Data/Scripts/SkirmishScripts.scb` and `MultiplayerScripts.scb`: both contribute to EXE CRC.
JSON contains only the fields for the inferred checksum scope.

`--installpath` supplies the INI data and script files and takes priority over discovery.
When omitted on Windows, the CLI uses the same HKLM keys as GeneralsGameCode's CMake
install targets, checking each key's 32-bit registry view before its 64-bit view:

| Game | Registry key under `SOFTWARE/Electronic Arts/EA Games` | Value, in lookup order |
| --- | --- | --- |
| Generals | `Generals`; `Command and Conquer The First Decade` | `InstallPath`; `gr_folder` |
| Zero Hour | `Command and Conquer Generals Zero Hour`; `Command and Conquer The First Decade`; `ZeroHour` | `InstallPath`; `zh_folder`; `installPath` |

Use `--game generals` or `--game generalsmd` when both games are installed. A recognized
absolute `--exe` can also identify the game for discovery. On other platforms, or when
no valid registry path is found, supply `--installpath` explicitly.

`--exe` may be an absolute path outside the installation. It defaults to `Generals.exe`;
when that default is the legacy launcher, the tool uses `Game.dat` instead.

`--sideload PATH` accepts a `.big` archive or directory as an additional game-folder
source. It is repeatable. Loose files from later sideload directories override earlier
ones; files inside `.big` archives follow case-insensitive archive filename order, with
the alphabetically first archive taking precedence.

`--mod PATH` accepts a `.big` archive or directory and overrides archive inputs.

