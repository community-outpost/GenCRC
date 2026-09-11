package gamecrc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/community-outpost/GenCRC/internal/checksum"
)

type archiveEntry struct {
	path    string
	archive string
	offset  int
	size    int
}

type FileSystem struct {
	root       string
	looseRoots []string
	entries    map[string]archiveEntry
}

// NewFileSystem indexes game archives while preserving loose-file precedence.
func NewFileSystem(root, game string, verbose bool) (*FileSystem, error) {
	vfs := &FileSystem{root: root, looseRoots: []string{root}, entries: make(map[string]archiveEntry)}
	var archives []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".big") {
			archives = append(archives, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(archives, func(i, j int) bool {
		return strings.ToLower(relative(root, archives[i])) < strings.ToLower(relative(root, archives[j]))
	})
	for _, archive := range archives {
		rel := strings.ToLower(relative(root, archive))
		if game == "generalsmd" && strings.HasSuffix(rel, `data\ini\inizh.big`) {
			continue
		}
		entries, err := readIndex(archive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "  indexed %s (%d files)\n", archive, len(entries))
		}
		for key, entry := range entries {
			if _, exists := vfs.entries[key]; !exists {
				vfs.entries[key] = entry
			}
		}
	}
	return vfs, nil
}

func readIndex(archive string) (map[string]archiveEntry, error) {
	data, err := os.ReadFile(archive)
	if err != nil {
		return nil, err
	}
	parsed, err := parseBigDirectory(data, archive)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]archiveEntry, len(parsed))
	for key, entry := range parsed {
		entries[key] = archiveEntry{path: entry.path, archive: archive, offset: entry.offset, size: entry.size}
	}
	return entries, nil
}

func (vfs *FileSystem) addArchive(archive string, mod bool) error {
	entries, err := readIndex(archive)
	if err != nil {
		return err
	}
	for key, entry := range entries {
		incumbent, exists := vfs.entries[key]
		if mod || !exists || strings.ToLower(filepath.Base(archive)) < strings.ToLower(filepath.Base(incumbent.archive)) {
			vfs.entries[key] = entry
		}
	}
	return nil
}

// AddSideload layers an archive or directory as game-folder content.
func (vfs *FileSystem) AddSideload(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return vfs.addArchive(path, false)
	}
	vfs.looseRoots = append(vfs.looseRoots, path)
	var archives []string
	err = filepath.WalkDir(path, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(file), ".big") {
			archives = append(archives, file)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(archives)
	for _, archive := range archives {
		if err := vfs.addArchive(archive, false); err != nil {
			return err
		}
	}
	return nil
}

// AddFallbackGeneralsRoot layers original Generals archives before sideloads.
func (vfs *FileSystem) AddFallbackGeneralsRoot(root string) error {
	var archives []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".big") {
			archives = append(archives, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(archives)
	for _, archive := range archives {
		if err := vfs.addArchive(archive, true); err != nil {
			return err
		}
	}
	return nil
}

func (vfs *FileSystem) AddMod(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return vfs.addArchive(path, true)
	}
	var archives []string
	err = filepath.WalkDir(path, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(file), ".big") {
			archives = append(archives, file)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(archives)
	for _, archive := range archives {
		if err := vfs.addArchive(archive, true); err != nil {
			return err
		}
	}
	return nil
}

func (vfs *FileSystem) Read(rel string) ([]byte, error) {
	// Later sideload roots override earlier roots and all archives.
	for index := len(vfs.looseRoots) - 1; index >= 0; index-- {
		loose := filepath.Join(vfs.looseRoots[index], filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/")))
		if data, err := os.ReadFile(loose); err == nil {
			return data, nil
		}
	}
	entry, found := vfs.entries[strings.ToLower(strings.ReplaceAll(rel, "/", `\`))]
	if !found {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(entry.archive)
	if err != nil {
		return nil, err
	}
	return data[entry.offset : entry.offset+entry.size], nil
}

func (vfs *FileSystem) FilesUnder(dir string) []string {
	prefix := strings.ToLower(strings.TrimSuffix(strings.ReplaceAll(dir, "/", `\`), `\`)) + `\`
	files := make(map[string]string)
	for _, root := range vfs.looseRoots {
		loose := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(dir, `\`, "/")))
		_ = filepath.WalkDir(loose, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".ini") {
				rel := relative(root, path)
				files[strings.ToLower(rel)] = rel
			}
			return nil
		})
	}
	for key, entry := range vfs.entries {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".ini") {
			if _, exists := files[key]; !exists {
				files[key] = entry.path
			}
		}
	}
	result := make([]string, 0, len(files))
	for _, path := range files {
		result = append(result, path)
	}
	return result
}

func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return strings.ReplaceAll(rel, "/", `\`)
}

func loadDirectory(vfs *FileSystem, path string, crc *checksum.BlockCRC, verbose bool) {
	read := func(file string) bool {
		data, err := vfs.Read(file)
		if err != nil {
			return false
		}
		checksum.INILines(data, crc.Add)
		if verbose {
			fmt.Fprintf(os.Stderr, "  %s\n", file)
		}
		return true
	}
	found := read(path + ".ini")
	files := vfs.FilesUnder(path)
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i]) < strings.ToLower(files[j]) })
	prefix := len(path) + 1
	for _, nested := range []bool{false, true} {
		for _, file := range files {
			isNested := strings.Contains(file[prefix:], `\`)
			if isNested == nested {
				found = read(file) || found
			}
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "warning: no INI content found for '%s'\n", path)
	}
}

var generalsMDOrder = [][2]string{
	{`Data\INI\Default\GameData`, `Data\INI\GameData`}, {`Data\INI\Default\Water`, ``}, {`Data\INI\Water`, ``}, {`Data\INI\Default\Weather`, ``}, {`Data\INI\Weather`, ``},
	{`Data\INI\Default\Science`, `Data\INI\Science`}, {`Data\INI\Default\Multiplayer`, `Data\INI\Multiplayer`}, {`Data\INI\Default\Terrain`, `Data\INI\Terrain`}, {`Data\INI\Default\Roads`, `Data\INI\Roads`},
	{``, `Data\INI\Rank`}, {`Data\INI\Default\PlayerTemplate`, `Data\INI\PlayerTemplate`}, {`Data\INI\Default\FXList`, `Data\INI\FXList`}, {``, `Data\INI\Weapon`}, {`Data\INI\Default\ObjectCreationList`, `Data\INI\ObjectCreationList`}, {``, `Data\INI\Locomotor`}, {`Data\INI\Default\SpecialPower`, `Data\INI\SpecialPower`}, {``, `Data\INI\DamageFX`}, {``, `Data\INI\Armor`}, {`Data\INI\Default\Object`, `Data\INI\Object`}, {`Data\INI\Default\Upgrade`, `Data\INI\Upgrade`}, {`Data\INI\Default\AIData`, `Data\INI\AIData`}, {`Data\INI\Default\Crate`, `Data\INI\Crate`},
}

// INICRC mirrors the engine's ordered core INI load sequence.
func INICRC(root, fallbackGeneralsRoot, game string, sideloads []string, mod string, verbose bool) (uint32, error) {
	vfs, err := NewFileSystem(root, game, verbose)
	if err != nil {
		return 0, err
	}
	if fallbackGeneralsRoot != "" {
		if err := vfs.AddFallbackGeneralsRoot(fallbackGeneralsRoot); err != nil {
			return 0, err
		}
	}
	order := generalsMDOrder
	if game == "generals" {
		order = append([][2]string(nil), order[:3]...)
		order = append(order, generalsMDOrder[5:]...)
	}
	crc := new(checksum.BlockCRC)
	for _, path := range order[0] {
		if path != "" {
			loadDirectory(vfs, path, crc, verbose)
		}
	}
	for _, sideload := range sideloads {
		if err := vfs.AddSideload(sideload); err != nil {
			return 0, err
		}
	}
	if mod != "" {
		if err := vfs.AddMod(mod); err != nil {
			return 0, err
		}
	}
	for _, pair := range order[1:] {
		for _, path := range pair {
			if path != "" {
				loadDirectory(vfs, path, crc, verbose)
			}
		}
	}
	return crc.Value(), nil
}

func ExeCRC(exe, root string, major, minor int) (uint32, error) {
	data, err := os.ReadFile(exe)
	if err != nil {
		return 0, err
	}
	crc := new(checksum.ByteCRC)
	crc.Add(data)
	crc.Add([]byte{byte(minor), byte(minor >> 8), byte(major), byte(major >> 8)})
	for _, name := range []string{"SkirmishScripts.scb", "MultiplayerScripts.scb"} {
		data, err := os.ReadFile(filepath.Join(root, "Data", "Scripts", name))
		if err == nil {
			crc.Add(data)
		} else {
			fmt.Fprintf(os.Stderr, "warning: %s not found, exeCRC will not match a real client's\n", name)
		}
	}
	return crc.Value(), nil
}

func DetectGame(root string) string {
	foundMD, foundBase := false, false
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if strings.EqualFold(entry.Name(), "INIZH.big") {
				foundMD = true
			}
			if strings.EqualFold(entry.Name(), "INI.big") {
				foundBase = true
			}
		}
		return nil
	})
	if foundMD {
		return "generalsmd"
	}
	if foundBase {
		return "generals"
	}
	return ""
}

func DetectGameFromExe(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return DetectGameBytes(data)
}

// DetectGameBytes identifies the game from executable strings without filesystem access.
func DetectGameBytes(data []byte) string {
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, "generalsmd") || strings.Contains(lower, "generals zero hour") || strings.Contains(lower, "generalszh") {
		return "generalsmd"
	}
	if strings.Contains(lower, "command and conquer generals") {
		return "generals"
	}
	return ""
}

func ResolveExe(root, requested string) string {
	if requested == "" {
		requested = "Generals.exe"
	}
	if filepath.IsAbs(requested) {
		return resolveName(filepath.Dir(requested), filepath.Base(requested), requested)
	}
	return resolveName(root, filepath.Base(requested), filepath.Join(root, requested))
}

func resolveName(directory, name, fallback string) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fallback
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), name) {
			return filepath.Join(directory, entry.Name())
		}
	}
	return fallback
}

func IsLauncher(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && IsLauncherBytes(data)
}

// IsLauncherBytes identifies the retail bootstrapper, not the game engine.
func IsLauncherBytes(data []byte) bool {
	return strings.Contains(string(data), "Launcher config file missing") && strings.Contains(string(data), "launcher.cfg")
}
