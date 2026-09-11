package installpath

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryPriority(t *testing.T) {
	primary, secondary := filepath.Join(t.TempDir(), "primary"), filepath.Join(t.TempDir(), "secondary")
	for _, tc := range []struct {
		name                      string
		missing32, missingPrimary bool
		want                      string
	}{
		{"32 bit first", false, false, primary}, {"64 bit fallback", true, false, secondary}, {"alternate key fallback", true, true, secondary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			root, game, err := discover("generalsmd", func(key, name string, view uint32) (string, error) {
				calls++
				if key == keys["generalsmd"][0].key {
					if tc.missingPrimary {
						return "missing", nil
					}
					if view == 0x0200 {
						if tc.missing32 {
							return "", fmt.Errorf("not found")
						}
						return primary, nil
					}
					return secondary, nil
				}
				return secondary, nil
			}, func(root string) bool { return root == primary || root == secondary })
			if err != nil || game != "generalsmd" || root != tc.want {
				t.Fatalf("%s %s %v", root, game, err)
			}
			if tc.name == "32 bit first" && calls != 1 {
				t.Fatalf("queried lower priority keys: %d", calls)
			}
		})
	}
}
func TestRegistryAmbiguityAndMissing(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name, game, want string
		valid            bool
	}{
		{"both installed", "", "both Generals", true}, {"missing requested game", "generals", "no installed game", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := discover(tc.game, func(string, string, uint32) (string, error) { return root, nil }, func(string) bool { return tc.valid })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
}
