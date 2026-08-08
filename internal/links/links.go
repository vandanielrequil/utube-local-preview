package links

import (
	"fmt"
	"os"
	"strings"
)

const PlaylistPrefix = "PLAYLIST_"

// Entry is one item from u-links.txt.
type Entry struct {
	URL      string
	Playlist bool
}

// ReadFile parses YouTube URLs separated by spaces and/or newlines.
// Prefix a URL with PLAYLIST_ to download the whole playlist/mix.
func ReadFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read links file %q: %w", path, err)
	}

	fields := strings.Fields(string(data))
	entries := make([]Entry, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		raw := strings.TrimSpace(field)
		if raw == "" {
			continue
		}

		playlist := false
		if strings.HasPrefix(raw, PlaylistPrefix) {
			playlist = true
			raw = strings.TrimPrefix(raw, PlaylistPrefix)
			raw = strings.TrimSpace(raw)
		}
		if raw == "" {
			continue
		}

		key := fmt.Sprintf("%t\n%s", playlist, raw)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, Entry{URL: raw, Playlist: playlist})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no links found in %q", path)
	}
	return entries, nil
}
