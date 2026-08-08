package youtube

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const formatSelector = "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best"

// Downloader downloads YouTube media via yt-dlp (same approach as instab/trailersReels).
type Downloader struct {
	YTDLP string
}

func NewDownloader() (*Downloader, error) {
	ytDlp, err := exec.LookPath("yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("yt-dlp not found in PATH")
	}
	return &Downloader{YTDLP: ytDlp}, nil
}

// Download saves a single video into outputDir as <videoId>.mp4.
// Existing non-empty files are skipped, matching instab behavior.
func (d *Downloader) Download(videoURL, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	videoURL = NormalizeURL(videoURL, false)
	videoID := VideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("could not extract video id from %q", videoURL)
	}

	outputPath := filepath.Join(outputDir, ToLowerNoSymbols(videoID)+".mp4")
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		fmt.Println("skip existing:", outputPath)
		return outputPath, nil
	}
	_ = os.Remove(outputPath)

	cmd := exec.Command(d.YTDLP,
		"--no-playlist",
		"-f", formatSelector,
		"--merge-output-format", "mp4",
		"-o", outputPath,
		videoURL,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("download:", videoURL, "->", outputPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed for %q: %w", videoURL, err)
	}

	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("downloaded file missing or empty: %s", outputPath)
	}
	return outputPath, nil
}

// DownloadPlaylist downloads a full playlist/mix into outputDir/<playlistName>/
// (or today's date if the title cannot be resolved). Returns paths of .mp4 files.
func (d *Downloader) DownloadPlaylist(playlistURL, outputDir string) (string, []string, error) {
	playlistURL = NormalizeURL(playlistURL, true)

	title, err := d.PlaylistTitle(playlistURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: playlist title: %v\n", err)
	}
	folder := SanitizeDirName(title)
	if folder == "" {
		folder = time.Now().Format("2006-01-02")
		fmt.Println("playlist folder fallback:", folder)
	}

	targetDir := filepath.Join(outputDir, folder)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("create playlist dir: %w", err)
	}

	outputTemplate := filepath.Join(targetDir, "%(id)s.%(ext)s")
	cmd := exec.Command(d.YTDLP,
		"--yes-playlist",
		"--ignore-errors",
		"-f", formatSelector,
		"--merge-output-format", "mp4",
		"-o", outputTemplate,
		playlistURL,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("download playlist:", playlistURL, "->", targetDir)
	if err := cmd.Run(); err != nil {
		// With --ignore-errors yt-dlp may still exit non-zero if some items failed.
		files, listErr := listMP4(targetDir)
		if listErr != nil || len(files) == 0 {
			return "", nil, fmt.Errorf("yt-dlp playlist failed for %q: %w", playlistURL, err)
		}
		fmt.Fprintf(os.Stderr, "warn: playlist finished with errors, got %d file(s): %v\n", len(files), err)
		return targetDir, files, nil
	}

	files, err := listMP4(targetDir)
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("playlist download produced no mp4 files in %s", targetDir)
	}
	return targetDir, files, nil
}

// PlaylistTitle asks yt-dlp for the playlist title.
func (d *Downloader) PlaylistTitle(playlistURL string) (string, error) {
	cmd := exec.Command(d.YTDLP,
		"--flat-playlist",
		"--print", "%(playlist_title)s",
		"--playlist-items", "1",
		playlistURL,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}

	title := strings.TrimSpace(stdout.String())
	// yt-dlp may print NA / empty when title is missing.
	if title == "" || strings.EqualFold(title, "NA") || strings.EqualFold(title, "null") {
		return "", fmt.Errorf("empty playlist title")
	}
	// Only keep the first line if multiple were printed.
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = strings.TrimSpace(title[:i])
	}
	return title, nil
}

// NormalizeURL accepts a full URL or bare video id.
// When asPlaylist is false, playlist/radio query params are stripped.
func NormalizeURL(raw string, asPlaylist bool) string {
	raw = strings.TrimSpace(raw)
	if id := VideoID(raw); id != "" && !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return "https://www.youtube.com/watch?v=" + id
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return raw
	}
	if asPlaylist {
		return raw
	}

	if id := VideoID(raw); id != "" {
		return "https://www.youtube.com/watch?v=" + id
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for _, key := range []string{"list", "start_radio", "index", "pp"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// SanitizeDirName makes a playlist title safe as a single directory name.
func SanitizeDirName(name string) string {
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
	// Windows MAX_PATH comfort for a single folder segment.
	const maxLen = 80
	if len(name) > maxLen {
		name = strings.TrimSpace(name[:maxLen])
		name = strings.Trim(name, " .")
	}
	return name
}

func listMP4(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.mp4"))
	if err != nil {
		return nil, fmt.Errorf("list mp4 in %s: %w", dir, err)
	}
	return matches, nil
}
