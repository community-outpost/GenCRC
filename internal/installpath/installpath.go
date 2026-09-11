package installpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type registryValue struct{ key, name string }

var keys = map[string][]registryValue{
	"generals":   {{`SOFTWARE\Electronic Arts\EA Games\Generals`, "InstallPath"}, {`SOFTWARE\Electronic Arts\EA Games\Command and Conquer The First Decade`, "gr_folder"}},
	"generalsmd": {{`SOFTWARE\Electronic Arts\EA Games\Command and Conquer Generals Zero Hour`, "InstallPath"}, {`SOFTWARE\Electronic Arts\EA Games\Command and Conquer The First Decade`, "zh_folder"}, {`SOFTWARE\Electronic Arts\EA Games\ZeroHour`, "installPath"}},
}

// Discover follows the game's CMake install-target registry priority, 32-bit view first.
func Discover(game string) (string, string, error) { return discover(game, readRegistry, isDirectory) }
func isDirectory(path string) bool                 { info, err := os.Stat(path); return err == nil && info.IsDir() }
func discover(game string, read func(string, string, uint32) (string, error), valid func(string) bool) (string, string, error) {
	if game != "" && game != "generals" && game != "generalsmd" {
		return "", "", fmt.Errorf("unknown game %q", game)
	}
	var roots []struct{ root, game string }
	for _, candidate := range []string{"generals", "generalsmd"} {
		if game != "" && candidate != game {
			continue
		}
		found := ""
		for _, key := range keys[candidate] {
			for _, view := range []uint32{0x0200, 0x0100} {
				root, err := read(key.key, key.name, view)
				root = strings.Trim(strings.TrimSpace(root), `"`)
				if err == nil && root != "" && filepath.IsAbs(root) && valid(root) {
					found = root
					break
				}
			}
			if found != "" {
				break
			}
		}
		if found != "" {
			roots = append(roots, struct{ root, game string }{found, candidate})
		}
	}
	if len(roots) == 0 {
		return "", "", fmt.Errorf("no installed game found in the Windows registry; pass --installpath <directory> (and --game generals|generalsmd when needed)")
	}
	if len(roots) > 1 {
		return "", "", fmt.Errorf("both Generals and Zero Hour are installed; pass --game generals|generalsmd or --installpath <directory>")
	}
	return roots[0].root, roots[0].game, nil
}
