package youtube

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	watchIDRe    = regexp.MustCompile(`(?i)(?:v=|/embed/|/v/|/shorts/|/live/)([a-zA-Z0-9_-]{11})`)
	youtuBeRe    = regexp.MustCompile(`(?i)youtu\.be/([a-zA-Z0-9_-]{11})`)
	bareIDRe     = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	playlistIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// VideoID extracts a YouTube video id from a URL or bare id.
func VideoID(raw string) string {
	raw = strings.TrimSpace(raw)
	if bareIDRe.MatchString(raw) {
		return raw
	}

	if match := youtuBeRe.FindStringSubmatch(raw); len(match) == 2 {
		return match[1]
	}
	if match := watchIDRe.FindStringSubmatch(raw); len(match) == 2 {
		return match[1]
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if id := parsed.Query().Get("v"); bareIDRe.MatchString(id) {
		return id
	}
	return ""
}

// PlaylistID extracts a playlist/list id from a YouTube URL.
func PlaylistID(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	id := parsed.Query().Get("list")
	if id == "" {
		return ""
	}
	if !playlistIDRe.MatchString(id) {
		return ""
	}
	return id
}
