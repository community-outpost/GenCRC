package gamecrc

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/community-outpost/GenCRC/internal/checksum"
)

type memoryEntry struct {
	path, archive string
	data          []byte
}

// MemorySource is one ordered sideload source. Archive is a standalone BIG;
// Files represents a folder, including loose files and any contained BIGs.
type MemorySource struct {
	Name    string
	Archive []byte
	Files   map[string][]byte
}

// MemoryArchive is one mod BIG. Mod folders are represented as a sorted slice.
type MemoryArchive struct {
	Name string
	Data []byte
}

func readMemoryIndex(data []byte, archive string) (map[string]memoryEntry, error) {
	parsed, err := parseBigDirectory(data, archive)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]memoryEntry, len(parsed))
	for key, entry := range parsed {
		entries[key] = memoryEntry{path: entry.path, archive: archive, data: data[entry.offset : entry.offset+entry.size]}
	}
	return entries, nil
}
func mergeMemory(target map[string]memoryEntry, source map[string]memoryEntry, baseline bool) {
	for key, entry := range source {
		incumbent, exists := target[key]
		if !exists || (!baseline && strings.ToLower(filepath.Base(entry.archive)) < strings.ToLower(filepath.Base(incumbent.archive))) {
			target[key] = entry
		}
	}
}
func loadMemoryDirectory(entries, loose map[string]memoryEntry, path string, crc *checksum.BlockCRC) {
	read := func(key string) (memoryEntry, bool) {
		if entry, ok := loose[key]; ok {
			return entry, true
		}
		entry, ok := entries[key]
		return entry, ok
	}
	if entry, ok := read(strings.ToLower(path + ".ini")); ok {
		checksum.INILines(entry.data, crc.Add)
	}
	prefix := strings.ToLower(path + `\`)
	var files []string
	for key := range entries {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".ini") {
			files = append(files, key)
		}
	}
	for key := range loose {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".ini") {
			files = append(files, key)
		}
	}
	files = uniqueStrings(files)
	sort.Strings(files)
	for _, nested := range []bool{false, true} {
		for _, file := range files {
			if strings.Contains(file[len(prefix):], `\`) == nested {
				entry, _ := read(file)
				checksum.INILines(entry.data, crc.Add)
			}
		}
	}
}

func uniqueStrings(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
func INICRCBytes(game string, baseline map[string][]byte, patches []MemorySource, mods []MemoryArchive) (uint32, error) {
	entries := make(map[string]memoryEntry)
	loose := make(map[string]memoryEntry)
	name := "INI.big"
	if game == "generalsmd" {
		name = "INIZH.big"
	}
	indexed, err := readMemoryIndex(baseline[name], name)
	if err != nil {
		return 0, err
	}
	mergeMemory(entries, indexed, true)
	crc := new(checksum.BlockCRC)
	for _, path := range generalsMDOrder[0] {
		if path != "" {
			loadMemoryDirectory(entries, nil, path, crc)
		}
	}
	for _, source := range patches {
		if len(source.Archive) != 0 {
			indexed, err = readMemoryIndex(source.Archive, source.Name)
			if err != nil {
				return 0, err
			}
			mergeMemory(entries, indexed, false)
			continue
		}
		archiveNames := make([]string, 0)
		for path := range source.Files {
			if strings.EqualFold(filepath.Ext(path), ".big") {
				archiveNames = append(archiveNames, path)
			}
		}
		sort.Strings(archiveNames)
		for _, archiveName := range archiveNames {
			indexed, err = readMemoryIndex(source.Files[archiveName], archiveName)
			if err != nil {
				return 0, err
			}
			mergeMemory(entries, indexed, false)
		}
		for path, data := range source.Files {
			if !strings.EqualFold(filepath.Ext(path), ".big") {
				normalized := strings.ReplaceAll(path, "/", `\`)
				loose[strings.ToLower(normalized)] = memoryEntry{path: normalized, data: data}
			}
		}
	}
	for _, mod := range mods {
		indexed, err = readMemoryIndex(mod.Data, mod.Name)
		if err != nil {
			return 0, err
		}
		for key, entry := range indexed {
			entries[key] = entry
		}
	}
	order := generalsMDOrder
	if game == "generals" {
		order = append([][2]string(nil), order[:3]...)
		order = append(order, generalsMDOrder[5:]...)
	}
	for _, pair := range order[1:] {
		for _, path := range pair {
			if path != "" {
				loadMemoryDirectory(entries, loose, path, crc)
			}
		}
	}
	return crc.Value(), nil
}
func ExeCRCBytes(exe []byte, major, minor int, skirmish, multiplayer []byte) uint32 {
	crc := new(checksum.ByteCRC)
	crc.Add(exe)
	crc.Add([]byte{byte(minor), byte(minor >> 8), byte(major), byte(major >> 8)})
	crc.Add(skirmish)
	crc.Add(multiplayer)
	return crc.Value()
}
