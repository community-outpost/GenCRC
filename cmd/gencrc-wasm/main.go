//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"syscall/js"

	"github.com/community-outpost/GenCRC/internal/gamecrc"
	"github.com/community-outpost/GenCRC/internal/version"
)

type fileMeta struct {
	Name string `json:"name"`
	Blob int    `json:"blob"`
}
type sourceMeta struct {
	Kind  string     `json:"kind"`
	Name  string     `json:"name"`
	Files []fileMeta `json:"files"`
}
type uploadMeta struct {
	Additional []sourceMeta `json:"additional"`
	Mod        *sourceMeta  `json:"mod"`
}

func bytes(value js.Value) []byte {
	data := make([]byte, value.Get("byteLength").Int())
	js.CopyBytesToGo(data, value)
	return data
}
func errorJSON(err error) string {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(data)
}

// Inspect before loading snapshots so an invalid upload gets actionable feedback.
func inspectExecutable(exe []byte) map[string]string {
	if gamecrc.IsLauncherBytes(exe) {
		return map[string]string{"error_code": "launcher_executable", "error": "This is the game launcher, not the game executable. Upload Game.dat from your Generals or Zero Hour installation instead."}
	}
	game := gamecrc.DetectGameBytes(exe)
	_, _, err := version.Extract(exe)
	if game == "" || err != nil {
		return map[string]string{"error_code": "unsupported_executable", "error": "This file could not be recognized as a supported Generals or Zero Hour game executable. Upload Game.dat from your game installation, not a launcher."}
	}
	return map[string]string{"game": game}
}
func getBlob(blobs js.Value, index int) ([]byte, error) {
	if index < 0 || index >= blobs.Length() {
		return nil, fmt.Errorf("upload manifest references invalid blob %d", index)
	}
	return bytes(blobs.Index(index)), nil
}
func decodeSources(raw string, blobs js.Value) ([]gamecrc.MemorySource, []gamecrc.MemoryArchive, error) {
	var meta uploadMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, nil, fmt.Errorf("invalid upload manifest: %w", err)
	}
	additional := make([]gamecrc.MemorySource, 0, len(meta.Additional))
	for _, source := range meta.Additional {
		switch source.Kind {
		case "big":
			if len(source.Files) != 1 {
				return nil, nil, fmt.Errorf("BIG source must contain one file")
			}
			data, err := getBlob(blobs, source.Files[0].Blob)
			if err != nil {
				return nil, nil, err
			}
			additional = append(additional, gamecrc.MemorySource{Name: source.Files[0].Name, Archive: data})
		case "folder":
			files := make(map[string][]byte, len(source.Files))
			for _, file := range source.Files {
				data, err := getBlob(blobs, file.Blob)
				if err != nil {
					return nil, nil, err
				}
				files[file.Name] = data
			}
			additional = append(additional, gamecrc.MemorySource{Name: source.Name, Files: files})
		default:
			return nil, nil, fmt.Errorf("invalid additional source kind %q", source.Kind)
		}
	}
	var mods []gamecrc.MemoryArchive
	if meta.Mod != nil {
		switch meta.Mod.Kind {
		case "big":
			if len(meta.Mod.Files) != 1 {
				return nil, nil, fmt.Errorf("mod BIG source must contain one file")
			}
			file := meta.Mod.Files[0]
			data, err := getBlob(blobs, file.Blob)
			if err != nil {
				return nil, nil, err
			}
			mods = []gamecrc.MemoryArchive{{Name: file.Name, Data: data}}
		case "folder":
			files := append([]fileMeta(nil), meta.Mod.Files...)
			sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name) })
			for _, file := range files {
				if !strings.EqualFold(filepath.Ext(file.Name), ".big") {
					continue
				}
				data, err := getBlob(blobs, file.Blob)
				if err != nil {
					return nil, nil, err
				}
				mods = append(mods, gamecrc.MemoryArchive{Name: file.Name, Data: data})
			}
		default:
			return nil, nil, fmt.Errorf("invalid mod source kind %q", meta.Mod.Kind)
		}
	}
	return additional, mods, nil
}
func calculate(_ js.Value, args []js.Value) any {
	if len(args) != 8 {
		return errorJSON(fmt.Errorf("invalid bridge arguments"))
	}
	game, exe := args[0].String(), bytes(args[1])
	if len(exe) != 0 {
		info := inspectExecutable(exe)
		if info["error"] != "" {
			data, _ := json.Marshal(info)
			return string(data)
		}
		game = info["game"]
	} else if game != "generals" && game != "generalsmd" {
		return errorJSON(fmt.Errorf("game is required when no executable is uploaded"))
	}
	additional, mods, err := decodeSources(args[2].String(), args[3])
	if err != nil {
		return errorJSON(err)
	}
	iniName, ini := args[4].String(), bytes(args[5])
	skirmish, multiplayer := bytes(args[6]), bytes(args[7])
	result := map[string]string{"game": game}
	if len(exe) != 0 {
		major, minor, err := version.Extract(exe)
		if err != nil {
			return errorJSON(err)
		}
		result["exe_crc"] = fmt.Sprintf("%08X", gamecrc.ExeCRCBytes(exe, major, minor, skirmish, multiplayer))
		result["version"] = version.Format(major, minor)
		if online := version.Online(exe); online != "" {
			result["generalsonline_version"] = online
		}
	}
	// A supplied baseline also supports retail-only requests without uploads.
	if len(ini) != 0 || len(additional) != 0 || len(mods) != 0 {
		iniCRC, err := gamecrc.INICRCBytes(game, map[string][]byte{iniName: ini}, additional, mods)
		if err != nil {
			return errorJSON(err)
		}
		result["ini_crc"] = fmt.Sprintf("%08X", iniCRC)
	}
	data, _ := json.Marshal(result)
	return string(data)
}
func main() {
	js.Global().Set("genCRCCalculate", js.FuncOf(calculate))
	js.Global().Set("genCRCInspectExecutable", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			return errorJSON(fmt.Errorf("invalid bridge arguments"))
		}
		data, _ := json.Marshal(inspectExecutable(bytes(args[0])))
		return string(data)
	}))
	js.Global().Set("genCRCDetectGame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			return ""
		}
		return gamecrc.DetectGameBytes(bytes(args[0]))
	}))
	select {}
}
