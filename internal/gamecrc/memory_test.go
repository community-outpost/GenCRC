package gamecrc

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGameBytes(t *testing.T) {
	if got := DetectGameBytes([]byte("prefix GeneralsZH suffix")); got != "generalsmd" {
		t.Fatalf("got %q", got)
	}
}

func TestExeCRCBytes(t *testing.T) {
	if got := ExeCRCBytes([]byte{1}, 1, 2, []byte{3}, []byte{4}); got != 146 {
		t.Fatalf("got %d, want 146", got)
	}
}

func testBIG(path string, contents []byte) []byte {
	name := []byte(path)
	headerSize := 16 + 8 + len(name) + 1
	data := make([]byte, headerSize+len(contents))
	copy(data, "BIGF")
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[8:12], 1)
	binary.BigEndian.PutUint32(data[12:16], uint32(headerSize))
	binary.BigEndian.PutUint32(data[16:20], uint32(headerSize))
	binary.BigEndian.PutUint32(data[20:24], uint32(len(contents)))
	copy(data[24:], name)
	copy(data[headerSize:], contents)
	return data
}

func TestINICRCBytesMatchesFileSystemImplementation(t *testing.T) {
	root := t.TempDir()
	baseline := testBIG(`Data\INI\GameData.ini`, []byte("Value = 1 ; comment\r\n"))
	if err := os.WriteFile(filepath.Join(root, "INI.big"), baseline, 0o600); err != nil {
		t.Fatal(err)
	}
	empty := make([]byte, 16)
	copy(empty, "BIGF")
	sideload := filepath.Join(root, "z-sideload.big")
	if err := os.WriteFile(sideload, empty, 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := INICRC(root, "", "generals", []string{sideload}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := INICRCBytes("generals", map[string][]byte{"INI.big": baseline}, []MemorySource{{Name: "z-sideload.big", Archive: empty}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("INICRCBytes() = %08X, want %08X", got, want)
	}
}

func TestINICRCBytesLayersSourcesLikeFileSystem(t *testing.T) {
	root := t.TempDir()
	baseline := testBIG(`Data\INI\Weapon.ini`, []byte("Damage = 1\n"))
	if err := os.WriteFile(filepath.Join(root, "INI.big"), baseline, 0o600); err != nil {
		t.Fatal(err)
	}

	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	for directory, value := range map[string]string{firstRoot: "Damage = 2\n", secondRoot: "Damage = 3\n"} {
		path := filepath.Join(directory, "Data", "INI")
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "Weapon.ini"), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	archiveA, archiveZ := testBIG(`Data\INI\Armor.ini`, []byte("Armor = A\n")), testBIG(`Data\INI\Armor.ini`, []byte("Armor = Z\n"))
	aPath, zPath := filepath.Join(t.TempDir(), "a.big"), filepath.Join(t.TempDir(), "z.big")
	if err := os.WriteFile(aPath, archiveA, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zPath, archiveZ, 0o600); err != nil {
		t.Fatal(err)
	}
	modData := testBIG(`Data\INI\Armor.ini`, []byte("Armor = MOD\n"))
	modPath := filepath.Join(t.TempDir(), "mod.big")
	if err := os.WriteFile(modPath, modData, 0o600); err != nil {
		t.Fatal(err)
	}

	want, err := INICRC(root, "", "generals", []string{zPath, firstRoot, aPath, secondRoot}, modPath, false)
	if err != nil {
		t.Fatal(err)
	}
	patches := []MemorySource{
		{Name: "z.big", Archive: archiveZ},
		{Name: "first", Files: map[string][]byte{`Data\INI\Weapon.ini`: []byte("Damage = 2\n")}},
		{Name: "a.big", Archive: archiveA},
		{Name: "second", Files: map[string][]byte{`Data\INI\Weapon.ini`: []byte("Damage = 3\n")}},
	}
	got, err := INICRCBytes("generals", map[string][]byte{"INI.big": baseline}, patches, []MemoryArchive{{Name: "mod.big", Data: modData}})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("INICRCBytes() = %08X, want %08X", got, want)
	}
}
