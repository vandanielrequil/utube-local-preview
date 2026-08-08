package youtube

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	watchIDRe  = regexp.MustCompile(`(?i)(?:v=|/embed/|/v/|/shorts/|/live/)([a-zA-Z0-9_-]{11})`)
	youtuBeRe  = regexp.MustCompile(`(?i)youtu\.be/([a-zA-Z0-9_-]{11})`)
	bareIDRe   = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
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
