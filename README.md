# GenVersion

Calculate legacy `exeCRC` and `iniCRC` values.

```shell
go run ./cmd/GenVersion --installpath PATH [--exe NAME|ABSOLUTE_PATH]
                         [--sideload PATH ...] [--mod PATH] [-v]
```

Build a Windows executable with `go build -o GenVersion-windows-x86_64.exe ./cmd/GenVersion`.

No dependencies beyond the Go standard library.

The tool detects the game from the selected executable, then falls back to `INIZH.big` or
`INI.big`.

`--installpath` supplies the INI data and script files. `--exe` may be an absolute path
outside that folder. It defaults to `Generals.exe`; when that is the legacy launcher, the
tool uses `Game.dat` instead.

`--sideload PATH` accepts a `.big` archive or directory as an additional game-folder
source. It is repeatable. Loose files from later sideload directories override earlier
ones; files inside `.big` archives follow case-insensitive archive filename order, with
the alphabetically first archive taking precedence.

`--mod PATH` accepts a `.big` archive or directory and overrides archive inputs.
