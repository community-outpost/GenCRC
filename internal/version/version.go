package version

import (
	"fmt"
	"regexp"
)

var onlinePattern = regexp.MustCompile(`(?i)([0-9]{6}(?:_QFE[0-9]+)?)[\x00-\x7F]{1,32}Version:ProductTitle`)

// Extract finds the compiler-specific Version::setVersion startup call.
func Extract(data []byte) (int, int, error) {
	var matches [][2]int
	for offset := range data {
		if major, minor, found := matchMSVC(data[offset:]); found {
			matches = append(matches, [2]int{major, minor})
		}
		if major, minor, found := matchVC6(data[offset:]); found {
			matches = append(matches, [2]int{major, minor})
		}
		if major, minor, found := matchOptimizedMSVC(data[offset:]); found {
			matches = append(matches, [2]int{major, minor})
		}
	}
	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("could not locate the Version::setVersion startup sequence")
	}
	if len(matches) != 1 {
		return 0, 0, fmt.Errorf("ambiguous Version::setVersion startup sequences")
	}
	return matches[0][0], matches[0][1], nil
}

func consumePush(data []byte) ([]byte, bool) {
	if len(data) >= 2 && data[0] == 0x6a {
		return data[2:], true
	}
	if len(data) >= 5 && data[0] == 0x68 {
		return data[5:], true
	}
	return nil, false
}

// matchMSVC handles the debug MSVC argument layout.
func matchMSVC(data []byte) (int, int, bool) {
	var ok bool
	if data, ok = consumePush(data); !ok {
		return 0, 0, false
	}
	if data, ok = consumePush(data); !ok || len(data) < 2 || data[0] != 0x6a {
		return 0, 0, false
	}
	minor := int(data[1])
	data = data[2:]
	if len(data) < 2 || data[0] != 0x6a {
		return 0, 0, false
	}
	major := int(data[1])
	data = data[2:]
	if len(data) < 12 || data[0] != 0xc6 || data[1] != 0x45 || data[3] != 0 || data[4] != 0x8b || data[5] != 0x0d || data[10] != 0xe8 {
		return 0, 0, false
	}
	return major, minor, true
}

// matchVC6 handles the legacy VC6 argument layout.
func matchVC6(data []byte) (int, int, bool) {
	if len(data) < 7 || data[0] != 0x8b || data[1] != 0x0d || data[6] < 0x50 || data[6] > 0x57 {
		return 0, 0, false
	}
	data, ok := consumePush(data[7:])
	if !ok || len(data) < 4 || data[0] != 0x6a || data[2] != 0x6a {
		return 0, 0, false
	}
	minor, major := int(data[1]), int(data[3])
	data = data[4:]
	if len(data) < 8 || data[0] != 0xc6 || data[1] != 0x45 || data[3] != 0 || data[4] != 0xe8 {
		return 0, 0, false
	}
	return major, minor, true
}

// matchOptimizedMSVC handles the optimized MSVC argument layout.
func matchOptimizedMSVC(data []byte) (int, int, bool) {
	var ok bool
	if data, ok = consumePush(data); !ok {
		return 0, 0, false
	}
	if data, ok = consumePush(data); !ok || len(data) < 2 || data[0] != 0x6a {
		return 0, 0, false
	}
	minor := int(data[1])
	data = data[2:]
	if len(data) < 12 || data[0] != 0xc6 || data[1] != 0x45 || data[3] != 0 || data[4] != 0x8b || data[5] != 0x0d || data[10] != 0x6a || len(data) < 17 || data[12] != 0xe8 {
		return 0, 0, false
	}
	return int(data[11]), minor, true
}

func Online(data []byte) string {
	match := onlinePattern.FindSubmatch(data)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func Format(major, minor int) string { return fmt.Sprintf("%d.%02d", major, minor) }
