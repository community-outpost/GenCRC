package retailsnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T) (Options, []Manifest, discoverFunc) {
	t.Helper()
	root := t.TempDir()
	options := Options{Output: filepath.Join(root, "output")}
	roots := map[string]string{}
	var baselines []Manifest
	for _, game := range []string{"generals", "generalsmd"} {
		roots[game] = filepath.Join(root, "install", game)
		ini := "INI.big"
		if game == "generalsmd" {
			ini = "INIZH.big"
		}
		baseline := Manifest{Schema: 1, Game: game}
		for _, name := range []string{ini, "SkirmishScripts.scb", "MultiplayerScripts.scb"} {
			data := []byte(game + "/" + name)
			hash := sha256.Sum256(data)
			baseline.Files = append(baseline.Files, File{Name: name, Bytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:])})
			write(t, sourcePath(roots[game], name), data)
		}
		baselines = append(baselines, baseline)
	}
	discover := func(game string) (string, string, error) { return roots[game], game, nil }
	return options, baselines, discover
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func realOperations() operations { return operations{copy: copyChecked, rename: os.Rename} }

func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		files[relative] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPreparationSelectsRoots(t *testing.T) {
	for _, name := range []string{"both discovered", "explicit roots", "mixed explicit and discovered", "one game"} {
		t.Run(name, func(t *testing.T) {
			options, baselines, discover := fixture(t)
			wantCalls := []string{"generals", "generalsmd"}
			switch name {
			case "explicit roots":
				options.GeneralsRoot, _, _ = discover("generals")
				options.ZeroHourRoot, _, _ = discover("generalsmd")
				wantCalls = nil
			case "mixed explicit and discovered":
				options.GeneralsRoot, _, _ = discover("generals")
				wantCalls = []string{"generalsmd"}
			case "one game":
				options.Game = "generalsmd"
				wantCalls = []string{"generalsmd"}
			}
			var calls []string
			selected, err := run(options, baselines, func(game string) (string, string, error) {
				calls = append(calls, game)
				return discover(game)
			}, realOperations())
			if err != nil || !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("calls=%v want=%v error=%v", calls, wantCalls, err)
			}
			if err := verifyOutput(options.Output, selected); err != nil {
				t.Fatal(err)
			}
			before := tree(t, options.Output)
			if _, err := run(options, baselines, discover, realOperations()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, tree(t, options.Output)) {
				t.Fatal("preparation must produce deterministic files and manifests")
			}
		})
	}
}

func TestPreparationFailurePreservesExistingSnapshots(t *testing.T) {
	for _, name := range []string{"missing last source", "mismatched last source", "source changed while staging", "stage write failed", "second replacement failed", "second backup failed"} {
		t.Run(name, func(t *testing.T) {
			options, baselines, discover := fixture(t)
			if _, err := run(options, baselines, discover, realOperations()); err != nil {
				t.Fatal(err)
			}
			// An older owned snapshot can contain stale files; failures must retain it.
			write(t, filepath.Join(options.Output, "generals", "old.big"), []byte("previous snapshot"))
			before := tree(t, options.Output)
			ops := realOperations()
			root, _, _ := discover("generalsmd")
			lastSource := sourcePath(root, "MultiplayerScripts.scb")
			switch name {
			case "missing last source":
				if err := os.Remove(lastSource); err != nil {
					t.Fatal(err)
				}
			case "mismatched last source":
				write(t, lastSource, []byte("modified installation"))
			case "source changed while staging":
				ops.copy = func(source, destination string, file File) error {
					write(t, source, []byte("changed after validation"))
					return copyChecked(source, destination, file)
				}
			case "stage write failed":
				ops.copy = func(string, string, File) error { return errors.New("injected write failure") }
			default:
				ops.rename = func(from, to string) error {
					if name == "second replacement failed" && filepath.Base(from) == "new-generalsmd" || name == "second backup failed" && filepath.Base(to) == "old-generalsmd" {
						return errors.New("injected rename failure")
					}
					return os.Rename(from, to)
				}
			}
			if _, err := run(options, baselines, discover, ops); err == nil {
				t.Fatal("expected failure")
			}
			if after := tree(t, options.Output); !reflect.DeepEqual(before, after) {
				t.Fatalf("existing snapshots changed after failed preparation: before=%v after=%v", before, after)
			}
			if _, err := os.Stat(filepath.Join(options.Output, transactionName)); !os.IsNotExist(err) {
				t.Fatalf("transaction was not cleaned: %v", err)
			}
		})
	}
}

func TestIncompleteRollbackRetainsRecoveryBackups(t *testing.T) {
	options, baselines, discover := fixture(t)
	if _, err := run(options, baselines, discover, realOperations()); err != nil {
		t.Fatal(err)
	}
	ops := realOperations()
	ops.rename = func(from, to string) error {
		if filepath.Base(from) == "new-generalsmd" || filepath.Base(from) == "old-generals" {
			return errors.New("injected failure")
		}
		return os.Rename(from, to)
	}
	if _, err := run(options, baselines, discover, ops); err == nil || !strings.Contains(err.Error(), "recovery files are preserved") {
		t.Fatalf("expected recovery instructions, got %v", err)
	}
	backup := filepath.Join(options.Output, transactionName, "old-generals", "INI.big")
	if err := verifyFile(backup, baselines[0].Files[0]); err != nil {
		t.Fatalf("old snapshot backup was lost: %v", err)
	}
	options.Verify = true
	if _, err := run(options, baselines, discover, realOperations()); err == nil || !strings.Contains(err.Error(), "needs recovery") {
		t.Fatalf("verification should block incomplete recovery: %v", err)
	}
}

func TestFirstPreparationRollbackAndActiveLock(t *testing.T) {
	for _, name := range []string{"new output rollback", "active preparation"} {
		t.Run(name, func(t *testing.T) {
			options, baselines, discover := fixture(t)
			ops := realOperations()
			if name == "active preparation" {
				write(t, filepath.Join(options.Output, transactionName, "recovery.txt"), []byte("keep"))
			} else {
				ops.rename = func(from, to string) error {
					if filepath.Base(from) == "new-generalsmd" {
						return errors.New("injected replacement failure")
					}
					return os.Rename(from, to)
				}
			}
			if _, err := run(options, baselines, discover, ops); err == nil {
				t.Fatal("expected preparation failure")
			}
			files := tree(t, options.Output)
			if name == "new output rollback" && len(files) != 0 {
				t.Fatalf("first preparation left a partial snapshot: %v", files)
			}
			if name == "active preparation" && files[filepath.Join(transactionName, "recovery.txt")] != "keep" {
				t.Fatalf("another operation's recovery files were changed: %v", files)
			}
		})
	}
}

func TestVerificationAndStaleFiles(t *testing.T) {
	for _, name := range []string{"valid", "modified bytes", "modified manifest", "stale file", "stale directory"} {
		t.Run(name, func(t *testing.T) {
			options, baselines, discover := fixture(t)
			if _, err := run(options, baselines, discover, realOperations()); err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(options.Output, "generals")
			switch name {
			case "modified bytes":
				write(t, filepath.Join(directory, "INI.big"), []byte("modified"))
			case "modified manifest":
				write(t, filepath.Join(directory, "manifest.json"), []byte(`{"schema":1,"game":"generals","files":[]}`))
			case "stale file":
				write(t, filepath.Join(directory, "obsolete.big"), []byte("stale"))
			case "stale directory":
				write(t, filepath.Join(directory, "old", "file"), []byte("stale"))
			}
			before := tree(t, options.Output)
			options.Verify = true
			_, err := run(options, baselines, func(string) (string, string, error) {
				t.Fatal("verify must never query the registry")
				return "", "", nil
			}, realOperations())
			if (err == nil) != (name == "valid") {
				t.Fatalf("unexpected verify error: %v", err)
			}
			if !reflect.DeepEqual(before, tree(t, options.Output)) {
				t.Fatal("verify wrote to output")
			}
			options.Verify = false
			if _, err := run(options, baselines, discover, realOperations()); err != nil {
				t.Fatal(err)
			}
			if err := verifyOutput(options.Output, baselines); err != nil {
				t.Fatalf("preparation did not replace stale snapshot state: %v", err)
			}
		})
	}
}

func TestUnsafeOutputIsRejected(t *testing.T) {
	for _, name := range []string{"same as source", "inside source", "source inside output", "unrelated existing files", "linked output game", "alias to source", "unselected root"} {
		t.Run(name, func(t *testing.T) {
			options, baselines, discover := fixture(t)
			root, _, _ := discover("generals")
			switch name {
			case "same as source":
				options.Output = root
			case "inside source":
				options.Output = filepath.Join(root, "snapshots")
			case "source inside output":
				options.Output = filepath.Dir(root)
			case "unrelated existing files":
				write(t, filepath.Join(options.Output, "generals", "important.txt"), []byte("keep"))
			case "linked output game", "alias to source":
				if err := os.MkdirAll(options.Output, 0755); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(options.Output, "generals")
				if err := os.Symlink(root, link); err != nil {
					t.Skipf("directory symlinks unavailable: %v", err)
				}
				if name == "alias to source" {
					options.Output = filepath.Join(link, "snapshots")
				}
			case "unselected root":
				options.Game, options.GeneralsRoot = "generalsmd", root
			}
			sourceBefore := tree(t, root)
			if _, err := run(options, baselines, discover, realOperations()); err == nil {
				t.Fatal("unsafe or conflicting output should fail")
			}
			if !reflect.DeepEqual(sourceBefore, tree(t, root)) {
				t.Fatal("game installation was modified")
			}
		})
	}
}

func TestBaselineLockContainsOnlyExpectedAssets(t *testing.T) {
	var lock struct{ Games []Manifest }
	if err := json.Unmarshal(baselineJSON, &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Games) != 2 {
		t.Fatalf("expected two game baselines, got %d", len(lock.Games))
	}
	for i, game := range []string{"generals", "generalsmd"} {
		manifest := lock.Games[i]
		if manifest.Schema != 1 || manifest.Game != game || len(manifest.Files) != 3 {
			t.Fatalf("invalid baseline: %+v", manifest)
		}
		ini := "INI.big"
		if game == "generalsmd" {
			ini = "INIZH.big"
		}
		names := []string{ini, "SkirmishScripts.scb", "MultiplayerScripts.scb"}
		for i, file := range manifest.Files {
			if file.Name != names[i] {
				t.Fatalf("unexpected asset %q, want %q", file.Name, names[i])
			}
			if filepath.Base(file.Name) != file.Name || file.Bytes <= 0 {
				t.Fatalf("unsafe baseline file: %+v", file)
			}
			hash, err := hex.DecodeString(file.SHA256)
			if err != nil || len(hash) != sha256.Size {
				t.Fatalf("invalid fingerprint: %+v", file)
			}
		}
	}
}
