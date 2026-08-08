package youtube

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TrackMeta holds naming and ID3 fields for a music file.
type TrackMeta struct {
	Artist string
	Track  string
	Title  string
	Album  string
	Year   string
	Source string // uploader/channel fallback
}

// ResolvedNames returns artist/title for filenames and tags.
// Empty strings mean the field is unknown (no "Unknown *" placeholders).
func (m TrackMeta) ResolvedNames() (artist, title string) {
	artist = strings.TrimSpace(m.Artist)
	title = strings.TrimSpace(m.Track)

	if title == "" {
		raw := strings.TrimSpace(m.Title)
		if left, right, ok := splitArtistTrack(raw); ok {
			if artist == "" || strings.EqualFold(artist, left) {
				artist = left
			}
			title = right
		} else {
			title = raw
		}
	}

	if artist == "" {
		artist = strings.TrimSpace(m.Source)
	}
	return artist, title
}

// FormatFileName builds "Artist - Title", or just Title/Artist if one side is missing.
func FormatFileName(meta TrackMeta) string {
	artist, title := meta.ResolvedNames()
	switch {
	case artist != "" && title != "":
		return SanitizeFileName(artist + " - " + title)
	case title != "":
		return SanitizeFileName(title)
	case artist != "":
		return SanitizeFileName(artist)
	default:
		return "unknown"
	}
}

// FormatMusicFileName is an alias of FormatFileName.
func FormatMusicFileName(meta TrackMeta) string {
	return FormatFileName(meta)
}

// TrackMetaForURL asks yt-dlp for metadata without downloading.
// Non-zero exit is tolerated when title/artist lines are still present.
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
		"--print", "%(album)s",
		"--print", "%(release_year)s",
		"--print", "%(upload_date)s",
		videoURL,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	lines := strings.Split(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "\n")
	for len(lines) < 8 {
		lines = append(lines, "")
	}

	year := cleanMetaValue(lines[6])
	if year == "" {
		if date := cleanMetaValue(lines[7]); len(date) >= 4 {
			year = date[:4]
		}
	}
	if _, err := strconv.Atoi(year); err != nil {
		year = ""
	}

	meta := TrackMeta{
		Artist: cleanMetaValue(lines[0]),
		Track:  cleanMetaValue(lines[1]),
		Title:  cleanMetaValue(lines[2]),
		Source: firstNonEmpty(cleanMetaValue(lines[3]), cleanMetaValue(lines[4])),
		Album:  cleanMetaValue(lines[5]),
		Year:   year,
	}
	if meta.Title == "" && meta.Track == "" && meta.Artist == "" {
		if runErr != nil {
			return TrackMeta{}, fmt.Errorf("yt-dlp metadata failed for %q: %w: %s", videoURL, runErr, strings.TrimSpace(stderr.String()))
		}
		return TrackMeta{}, fmt.Errorf("yt-dlp metadata empty for %q", videoURL)
	}
	return meta, nil
}

// ResolveTrackMeta loads naming/tags for a video URL (no sidecar files).
func (d *Downloader) ResolveTrackMeta(videoPath, videoURL string) TrackMeta {
	meta, err := d.TrackMetaForURL(videoURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: metadata: %v\n", err)
		base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
		return TrackMeta{Title: base}
	}
	return meta
}

func cleanMetaValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "NA") || strings.EqualFold(s, "null") || strings.EqualFold(s, "none") {
		return ""
	}
	return s
}

func splitArtistTrack(title string) (string, string, bool) {
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
