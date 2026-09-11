// Package retailsnapshot prepares raw INI/SCB copies from a known fingerprint
// set. These are deployment inputs, not intermediate checksum-state snapshots.
package retailsnapshot

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/community-outpost/GenCRC/internal/installpath"
)

//go:embed baseline-lock.json
var baselineJSON []byte

type File struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Schema int    `json:"schema"`
	Game   string `json:"game"`
	Files  []File `json:"files"`
}

type Options struct {
	Game         string
	GeneralsRoot string
	ZeroHourRoot string
	Output       string
	Verify       bool
}

const transactionName = ".gencrc-prepare.lock"

type discoverFunc func(string) (string, string, error)
type operations struct {
	copy   func(string, string, File) error
	rename func(string, string) error
}

// Run validates against the embedded baseline lock. Explicit roots take
// precedence over registry discovery. Verification never queries the registry.
func Run(options Options) ([]Manifest, error) {
	var lock struct {
		Schema int        `json:"schema"`
		Games  []Manifest `json:"games"`
	}
	if err := json.Unmarshal(baselineJSON, &lock); err != nil {
		return nil, fmt.Errorf("read embedded baseline lock: %w", err)
	}
	if lock.Schema != 1 {
		return nil, fmt.Errorf("unsupported baseline lock schema %d", lock.Schema)
	}
	return run(options, lock.Games, installpath.Discover, operations{copy: copyChecked, rename: os.Rename})
}

func run(options Options, baselines []Manifest, discover discoverFunc, ops operations) ([]Manifest, error) {
	selected, err := selectGames(options.Game, baselines)
	if err != nil {
		return nil, err
	}
	if options.Output == "" {
		return nil, errors.New("snapshot output directory is required")
	}
	output, err := canonicalPath(options.Output)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	if filepath.Dir(output) == output {
		return nil, errors.New("snapshot output cannot be a filesystem root")
	}
	if options.Verify {
		if options.GeneralsRoot != "" || options.ZeroHourRoot != "" {
			return nil, errors.New("--verify checks prepared snapshots; do not supply installation roots")
		}
		return selected, verifyOutput(output, selected)
	}
	if options.Game == "generals" && options.ZeroHourRoot != "" || options.Game == "generalsmd" && options.GeneralsRoot != "" {
		return nil, errors.New("an installation root was supplied for a game excluded by --game")
	}

	// Validate every source before creating output or changing existing snapshots.
	roots := make(map[string]string, len(selected))
	for _, baseline := range selected {
		root := options.GeneralsRoot
		flag := "--generals-root"
		if baseline.Game == "generalsmd" {
			root, flag = options.ZeroHourRoot, "--zero-hour-root"
		}
		if root == "" {
			var detected string
			root, detected, err = discover(baseline.Game)
			if err != nil {
				return nil, fmt.Errorf("locate %s: pass %s <directory> if registry discovery is unavailable: %w", baseline.Game, flag, err)
			}
			if detected != baseline.Game {
				return nil, fmt.Errorf("registry discovery returned %s for %s", detected, baseline.Game)
			}
		}
		root, err = canonicalPath(root)
		if err != nil {
			return nil, fmt.Errorf("resolve %s installation: %w", baseline.Game, err)
		}
		if inside(root, output) || inside(output, root) {
			return nil, fmt.Errorf("snapshot output %q must not overlap installation %q", output, root)
		}
		for _, file := range baseline.Files {
			source := sourcePath(root, file.Name)
			resolved, resolveErr := filepath.EvalSymlinks(source)
			if resolveErr != nil {
				return nil, fmt.Errorf("required %s source %q: %w", baseline.Game, source, resolveErr)
			}
			if !inside(root, resolved) {
				return nil, fmt.Errorf("source %q resolves outside installation %q", source, root)
			}
			if err := verifyFile(source, file); err != nil {
				return nil, err
			}
		}
		roots[baseline.Game] = root
	}
	if err := prepare(output, selected, roots, ops); err != nil {
		return nil, err
	}
	return selected, nil
}

func selectGames(game string, baselines []Manifest) ([]Manifest, error) {
	if game != "" && game != "generals" && game != "generalsmd" {
		return nil, fmt.Errorf("unknown game %q; use generals or generalsmd", game)
	}
	var selected []Manifest
	for _, baseline := range baselines {
		if baseline.Game == game || game == "" {
			selected = append(selected, baseline)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no baseline available for %q", game)
	}
	return selected, nil
}

func sourcePath(root, name string) string {
	if strings.HasSuffix(name, ".scb") {
		return filepath.Join(root, "Data", "Scripts", name)
	}
	return filepath.Join(root, name)
}

func verifyFile(path string, expected File) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read required snapshot file %q: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot input %q is not a readable regular file", path)
	}
	hash := sha256.New()
	bytes, err := io.Copy(hash, file)
	if err != nil {
		return fmt.Errorf("hash %q: %w", path, err)
	}
	if bytes != expected.Bytes || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
		return fmt.Errorf("baseline fingerprint mismatch for %q; expected %d bytes, SHA-256 %s; installation may be modified or a different release", path, expected.Bytes, expected.SHA256)
	}
	return nil
}

func verifyOutput(output string, selected []Manifest) error {
	if _, err := os.Lstat(filepath.Join(output, transactionName)); !os.IsNotExist(err) {
		return fmt.Errorf("snapshot preparation is active or needs recovery: %s", filepath.Join(output, transactionName))
	}
	for _, baseline := range selected {
		directory := filepath.Join(output, baseline.Game)
		if err := requireDirectory(directory); err != nil {
			return err
		}
		manifest, err := readManifest(filepath.Join(directory, "manifest.json"))
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(manifest, baseline) {
			return fmt.Errorf("manifest for %s does not match the baseline lock; prepare snapshots again", baseline.Game)
		}
		allowed := map[string]bool{"manifest.json": true, ".gitkeep": true}
		for _, file := range baseline.Files {
			allowed[file.Name] = true
			if err := verifyFile(filepath.Join(directory, file.Name), file); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !allowed[entry.Name()] || !entry.Type().IsRegular() {
				return fmt.Errorf("unexpected snapshot entry %q; prepare snapshots again to remove stale files", filepath.Join(directory, entry.Name()))
			}
		}
	}
	return nil
}

func readManifest(path string) (Manifest, error) {
	var manifest Manifest
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &manifest)
	}
	if err != nil {
		return manifest, fmt.Errorf("read snapshot manifest %q: %w", path, err)
	}
	return manifest, nil
}

func copyChecked(source, destination string, expected File) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	// Recheck staged bytes: the source may have changed after initial validation.
	return verifyFile(destination, expected)
}

// canonicalPath resolves existing symlink ancestors, including when the final
// output directory does not exist yet. This makes overlap checks meaningful.
func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) || filepath.Dir(absolute) == absolute {
		return "", err
	}
	parent, err := canonicalPath(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

func inside(root, path string) bool {
	if runtime.GOOS == "windows" {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func requireDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("read snapshot directory %q: %w", path, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("snapshot path %q must be a directory, not a link or file", path)
	}
	return nil
}
