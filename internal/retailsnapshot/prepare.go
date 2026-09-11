package retailsnapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type replacement struct {
	target, staged, backup string
	hadPrevious, installed bool
}

func prepare(output string, selected []Manifest, roots map[string]string, ops operations) (result error) {
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	transaction := filepath.Join(output, transactionName)
	// Atomic directory creation excludes another preparer. On failed rollback the
	// directory remains with backups, and both preparation and verification stop.
	if err := os.Mkdir(transaction, 0700); err != nil {
		return fmt.Errorf("cannot start snapshot preparation; check for an active operation or recovery files in %q: %w", transaction, err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			result = errors.Join(result, removeTransaction(output, transaction))
		}
	}()

	var replacements []*replacement
	for _, baseline := range selected {
		target := filepath.Join(output, baseline.Game)
		if err := checkReplaceable(target, baseline.Game); err != nil {
			return err
		}
		staged := filepath.Join(transaction, "new-"+baseline.Game)
		if err := os.Mkdir(staged, 0755); err != nil {
			return err
		}
		for _, file := range baseline.Files {
			if err := ops.copy(sourcePath(roots[baseline.Game], file.Name), filepath.Join(staged, file.Name), file); err != nil {
				return fmt.Errorf("stage %s/%s: %w", baseline.Game, file.Name, err)
			}
		}
		data, err := json.MarshalIndent(baseline, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(staged, "manifest.json"), append(data, '\n'), 0644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(staged, ".gitkeep"), []byte("\n"), 0644); err != nil {
			return err
		}
		replacements = append(replacements, &replacement{target: target, staged: staged, backup: filepath.Join(transaction, "old-"+baseline.Game)})
	}

	// Every selected game is now staged and checked. Keep old directories intact
	// until all replacements succeed; ordinary filesystem errors roll them back.
	for _, replacement := range replacements {
		if _, err := os.Lstat(replacement.target); err == nil {
			if err := ops.rename(replacement.target, replacement.backup); err != nil {
				result = fmt.Errorf("back up snapshot %q: %w", replacement.target, err)
				break
			}
			replacement.hadPrevious = true
		} else if !os.IsNotExist(err) {
			result = err
			break
		}
		if err := ops.rename(replacement.staged, replacement.target); err != nil {
			result = fmt.Errorf("replace snapshot %q: %w", replacement.target, err)
			break
		}
		replacement.installed = true
	}
	if result != nil {
		for i := len(replacements) - 1; i >= 0; i-- {
			replacement := replacements[i]
			if replacement.installed {
				if err := ops.rename(replacement.target, replacement.staged); err != nil {
					cleanup = false
					result = errors.Join(result, fmt.Errorf("rollback new snapshot %q: %w", replacement.target, err))
					continue
				}
			}
			if replacement.hadPrevious {
				if err := ops.rename(replacement.backup, replacement.target); err != nil {
					cleanup = false
					result = errors.Join(result, fmt.Errorf("restore snapshot %q: %w", replacement.target, err))
				}
			}
		}
		if !cleanup {
			result = errors.Join(result, fmt.Errorf("automatic rollback was incomplete; recovery files are preserved in %q; do not deploy before restoring them", transaction))
		}
	}
	return result
}

// Only replace a directory already identifiable as a generated snapshot (or the
// initial tracked .gitkeep placeholder). Never discard an unrelated directory.
func checkReplaceable(path, game string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}
	if err := requireDirectory(path); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 || len(entries) == 1 && entries[0].Name() == ".gitkeep" && entries[0].Type().IsRegular() {
		return nil
	}
	manifest, err := readManifest(filepath.Join(path, "manifest.json"))
	if err != nil || manifest.Schema != 1 || manifest.Game != game {
		return fmt.Errorf("refusing to replace unrecognized snapshot directory %q; select a separate output directory", path)
	}
	return nil
}

func removeTransaction(output, transaction string) error {
	// Only this invocation's successfully created, direct child may be removed.
	// RemoveAll does not follow symlinks encountered inside the owned directory.
	resolved, err := canonicalPath(transaction)
	if err != nil {
		return err
	}
	if resolved != transaction || filepath.Dir(resolved) != output || filepath.Base(resolved) != transactionName {
		return fmt.Errorf("refusing to clean transaction outside verified output: %q", transaction)
	}
	if err := requireDirectory(transaction); err != nil {
		return err
	}
	if err := os.RemoveAll(transaction); err != nil {
		return fmt.Errorf("snapshot files were preserved but transaction cleanup failed at %q: %w", transaction, err)
	}
	return nil
}
