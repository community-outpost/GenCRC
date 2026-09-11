package gamecrc

import "testing"

func TestIsLauncherBytes(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want bool
	}{
		{"retail launcher", "MZ\x00Launcher config file missing\x00launcher.cfg", true},
		{"config reference alone", "launcher.cfg", false},
		{"message alone", "Launcher config file missing", false},
		{"game filename is not identity", "Generals.exe", false},
		{"empty", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLauncherBytes([]byte(tt.data)); got != tt.want {
				t.Fatalf("IsLauncherBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}
