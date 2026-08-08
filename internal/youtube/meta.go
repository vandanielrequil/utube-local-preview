package youtube

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// TrackMeta holds naming fields for a music file.
type TrackMeta struct {
	Artist string
	Track  string
	Title  string
	Source string // uploader/channel fallback info
}

// TrackMetaForURL asks yt-dlp for artist/track metadata (no download).
func (d *Downloader) TrackMetaForURL(videoURL string) (TrackMeta, error) {
	videoURL = NormalizeURL(videoURL, false)
	cmd := exec.Command(d.YTDLP,
		"--skip-download",
		"--no-playlist",
		"--print", "%(artist)s",
		"--print", "%(track)s",
		"--print", "%(title)s",
		"--print", "%(uploader)s",
		"--print", "%(channel)s",
		videoURL,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return TrackMeta{}, fmt.Errorf("yt-dlp metadata failed for %q: %w: %s", videoURL, err, strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "\n")
	for len(lines) < 5 {
		lines = append(lines, "")
	}
	clean := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "NA") || strings.EqualFold(s, "null") {
			return ""
		}
		return s
	}

	return TrackMeta{
		Artist: clean(lines[0]),
		Track:  clean(lines[1]),
		Title:  clean(lines[2]),
		Source: firstNonEmpty(clean(lines[3]), clean(lines[4])),
	}, nil
}

// FormatMusicFileName builds "Artist - Track" suitable as a file base name.
func FormatMusicFileName(meta TrackMeta) string {
	artist := strings.TrimSpace(meta.Artist)
	track := firstNonEmpty(strings.TrimSpace(meta.Track), strings.TrimSpace(meta.Title))

	if artist == "" && track != "" {
		if left, right, ok := splitArtistTrack(track); ok {
			artist, track = left, right
		}
	}
	if artist == "" {
		artist = firstNonEmpty(meta.Source, "Unknown Artist")
	}
	if track == "" {
		track = "Unknown Title"
	}

	return SanitizeFileName(artist + " - " + track)
}

func splitArtistTrack(title string) (string, string, bool) {
	// Prefer " - " (common YouTube music naming).
	parts := strings.SplitN(title, " - ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SanitizeFileName makes a single file base name safe for Windows/macOS/Linux.
func SanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"<", " ",
		">", " ",
		":", " ",
		`"`, " ",
		"/", " ",
		"\\", " ",
		"|", " ",
		"?", " ",
		"*", " ",
	)
	name = replacer.Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, " .")
	if name == "" || name == "." || name == ".." {
		return ""
	}
	const maxLen = 120
	if len(name) > maxLen {
		name = strings.TrimSpace(name[:maxLen])
		name = strings.Trim(name, " .")
	}
	return name
}
