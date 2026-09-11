package gamecrc

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// bigDirectoryEntry is one parsed BIG archive directory record, positioned
// within the archive's own byte range (not yet tied to a source: disk offset
// or in-memory slice is the caller's concern).
type bigDirectoryEntry struct {
	path          string
	offset, size int
}

// parseBigDirectory reads a BIG archive's header and directory table from its
// raw bytes. label identifies the archive in error messages.
func parseBigDirectory(data []byte, label string) (map[string]bigDirectoryEntry, error) {
	if len(data) < 16 || string(data[:4]) != "BIGF" {
		return nil, fmt.Errorf("%s: invalid BIG header", label)
	}
	count, pos := int(binary.BigEndian.Uint32(data[8:12])), 16
	entries := make(map[string]bigDirectoryEntry, count)
	for range count {
		if pos+8 > len(data) {
			return nil, fmt.Errorf("%s: truncated BIG directory", label)
		}
		offset, size := int(binary.BigEndian.Uint32(data[pos:])), int(binary.BigEndian.Uint32(data[pos+4:]))
		pos += 8
		end := pos
		for end < len(data) && data[end] != 0 {
			end++
		}
		if end == len(data) || offset < 0 || size < 0 || offset > len(data)-size {
			return nil, fmt.Errorf("%s: invalid BIG directory entry", label)
		}
		path := strings.ReplaceAll(string(data[pos:end]), "/", `\`)
		entries[strings.ToLower(path)] = bigDirectoryEntry{path: path, offset: offset, size: size}
		pos = end + 1
	}
	return entries, nil
}
