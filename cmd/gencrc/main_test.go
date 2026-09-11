package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, root, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
func binaryFixture() []byte {
	return append([]byte("generalsmd\x00"), []byte{0x6a, 0, 0x6a, 0, 0x6a, 4, 0x6a, 1, 0xc6, 0x45, 0, 0, 0x8b, 0x0d, 0, 0, 0, 0, 0xe8, 0}...)
}
func TestModeResourceBoundaries(t *testing.T) {
	for _, mode := range []string{"ini", "exe", "both"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if mode != "exe" {
				fixture(t, root, "Data/INI/Default/GameData.ini", []byte("GameData\nEnd\n"))
			}
			if mode != "ini" {
				fixture(t, root, "Generals.exe", binaryFixture())
				fixture(t, root, "Data/Scripts/SkirmishScripts.scb", []byte("skirmish"))
				fixture(t, root, "Data/Scripts/MultiplayerScripts.scb", []byte("multiplayer"))
			}
			var out, stderr bytes.Buffer
			discover := func(string) (string, string, error) {
				t.Fatal("explicit root must bypass registry")
				return "", "", nil
			}
			args := []string{"--game", "generalsmd", "--installpath", root}
			if mode != "ini" {
				args = append(args, "--exe", "Generals.exe")
			}
			if mode != "exe" {
				args = append(args, "--sideload", t.TempDir())
			}
			if err := run(args, &out, &stderr, discover); err != nil {
				t.Fatal(err)
			}
			var result map[string]string
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if (result["exe_crc"] != "") != (mode != "ini") {
				t.Fatalf("EXE field: %v", result)
			}
			if (result["ini_crc"] != "") != (mode != "exe") {
				t.Fatalf("INI field: %v", result)
			}
			if mode == "ini" && (result["version"] != "" || result["exe"] != "") {
				t.Fatalf("unexpected executable metadata: %v", result)
			}
		})
	}
}
func TestErrorsAndDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{

		{"bad game", []string{"--game", "unknown"}, "--game"},

		{"ambiguous registry", nil, "both games"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, &bytes.Buffer{}, &bytes.Buffer{}, func(string) (string, string, error) { return "", "", fmt.Errorf("both games installed") })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
	root := t.TempDir()
	fixture(t, root, "Generals.exe", binaryFixture())
	discover := func(game string) (string, string, error) {
		if game != "generalsmd" {
			t.Fatalf("game = %s", game)
		}
		return root, "generalsmd", nil
	}
	err := run([]string{"--exe", "Generals.exe", "--game", "generalsmd"}, &bytes.Buffer{}, &bytes.Buffer{}, discover)
	if err == nil || !strings.Contains(err.Error(), "SkirmishScripts.scb") {
		t.Fatalf("missing SCB: %v", err)
	}
	err = run([]string{"--exe", "Generals.exe", "--installpath", root, "--game", "generals"}, &bytes.Buffer{}, &bytes.Buffer{}, discover)
	if err == nil || !strings.Contains(err.Error(), "game mismatch") {
		t.Fatalf("EXE mismatch: %v", err)
	}
	fixture(t, root, "INIZH.big", nil)
	err = run([]string{"--sideload", t.TempDir(), "--installpath", root, "--game", "generals"}, &bytes.Buffer{}, &bytes.Buffer{}, discover)
	if err == nil || !strings.Contains(err.Error(), "game mismatch") {
		t.Fatalf("root mismatch: %v", err)
	}
}
func TestAbsoluteExeSelectsRegistryGame(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(t.TempDir(), "Game.dat")
	fixture(t, filepath.Dir(exe), "Game.dat", binaryFixture())
	for _, name := range []string{"SkirmishScripts.scb", "MultiplayerScripts.scb"} {
		fixture(t, root, "Data/Scripts/"+name, []byte("scripts"))
	}
	calls := 0
	err := run([]string{"--exe", exe}, &bytes.Buffer{}, &bytes.Buffer{}, func(game string) (string, string, error) {
		calls++
		if game != "generalsmd" {
			t.Fatalf("game=%s", game)
		}
		return root, game, nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("calls %d, err %v", calls, err)
	}
}

func TestInputInferenceAndDefaultLauncher(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inputs  []string
		wantEXE bool
	}{
		{"installed default", nil, true},
		{"mod only", []string{"--mod"}, false},
		{"fallback only", []string{"--fallback-generals-root"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fixture(t, root, "Generals.exe", []byte("Launcher config file missing launcher.cfg"))
			fixture(t, root, "Game.dat", binaryFixture())
			for _, name := range []string{"SkirmishScripts.scb", "MultiplayerScripts.scb"} {
				fixture(t, root, "Data/Scripts/"+name, []byte("scripts"))
			}
			args := []string{"--game", "generalsmd", "--installpath", root}
			if len(tc.inputs) > 0 {
				args = append(args, tc.inputs[0], t.TempDir())
			}
			var out bytes.Buffer
			if err := run(args, &out, &bytes.Buffer{}, func(string) (string, string, error) { t.Fatal("unexpected discovery"); return "", "", nil }); err != nil {
				t.Fatal(err)
			}
			var result map[string]string
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["ini_crc"] == "" || (result["exe_crc"] != "") != tc.wantEXE {
				t.Fatalf("result: %v", result)
			}
			if tc.wantEXE && result["exe"] != "Game.dat" {
				t.Fatalf("launcher not resolved: %v", result)
			}
		})
	}
}
