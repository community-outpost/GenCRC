//go:build !windows

package installpath

import "fmt"

func readRegistry(key, name string, view uint32) (string, error) {
	return "", fmt.Errorf("Windows registry discovery is unavailable on this platform; pass --installpath")
}
