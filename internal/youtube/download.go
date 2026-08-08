package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	formatSelector = "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best"
	maxAttempts    = 3
	retryDelay     = 3 * time.Second
)

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

// Download saves a single video into outputDir as "Artist - Title.mp4".
func (d *Downloader) Download(videoURL, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	videoURL = NormalizeURL(videoURL, false)
	videoID := VideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("could not extract video id from %q", videoURL)
	}

	tempPath := filepath.Join(outputDir, videoID+".mp4")
	if named := findNamedVideo(outputDir, videoID); named != "" {
		fmt.Println("skip existing:", named)
		return named, nil
	}
	if info, err := os.Stat(tempPath); err == nil && info.Size() > 0 {
		return d.renameByMeta(tempPath, videoURL)
	}

	if err := d.downloadWithRetry(videoURL, tempPath); err != nil {
		return "", err
	}
	removeSidecars(tempPath)
	return d.renameByMeta(tempPath, videoURL)
}

func (d *Downloader) downloadWithRetry(videoURL, outputPath string) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_ = os.Remove(outputPath)
		removeSidecars(outputPath)
		fmt.Printf("download (attempt %d/%d): %s -> %s\n", attempt, maxAttempts, videoURL, outputPath)
		lastErr = d.runDownload(videoURL, outputPath)
		if lastErr == nil {
			removeSidecars(outputPath)
			return nil
		}
		fmt.Fprintf(os.Stderr, "warn: download failed: %v\n", lastErr)
		if attempt < maxAttempts {
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("yt-dlp failed for %q after %d attempts: %w", videoURL, maxAttempts, lastErr)
}

func (d *Downloader) runDownload(videoURL, outputPath string) error {
	cmd := exec.Command(d.YTDLP,
		"--no-playlist",
		"--no-write-info-json",
		"-f", formatSelector,
		"--merge-output-format", "mp4",
		"-o", outputPath,
		videoURL,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("downloaded file missing or empty: %s", outputPath)
	}
	return nil
}

// DownloadPlaylist downloads a full playlist/mix into outputDir/<playlistName>/.
// Videos are renamed to "Artist - Title.mp4". Failed items are retried.
func (d *Downloader) DownloadPlaylist(playlistURL, outputDir string) (string, []string, error) {
	playlistURL = NormalizeURL(playlistURL, true)
	playlistID := PlaylistID(playlistURL)
	if playlistID == "" {
		return "", nil, fmt.Errorf("could not extract playlist id from %q", playlistURL)
	}

	title, err := d.PlaylistTitle(playlistURL)
	if err != nil {
		return "", nil, fmt.Errorf("playlist unavailable %q: %w", playlistURL, err)
	}
	folder := SanitizeDirName(title)
	if folder == "" {
		folder = SanitizeDirName(playlistID)
		fmt.Println("playlist folder fallback: id", folder)
	}
	if folder == "" {
		folder = time.Now().Format("2006-01-02")
		fmt.Println("playlist folder fallback: date", folder)
	}

	targetDir := filepath.Join(outputDir, folder)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("create playlist dir: %w", err)
	}

	ids, err := d.PlaylistVideoIDs(playlistURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: playlist id list: %v\n", err)
	}

	outputTemplate := filepath.Join(targetDir, "%(id)s.%(ext)s")
	cmd := exec.Command(d.YTDLP,
		"--yes-playlist",
		"--ignore-errors",
		"--no-overwrites",
		"--no-write-info-json",
		"-f", formatSelector,
		"--merge-output-format", "mp4",
		"-o", outputTemplate,
		playlistURL,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("download playlist:", playlistURL, "->", targetDir)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warn: playlist pass finished with errors: %v\n", err)
	}
	cleanupInfoJSON(targetDir)

	if err := d.retryMissingVideos(targetDir, ids); err != nil {
		fmt.Fprintf(os.Stderr, "warn: retries: %v\n", err)
	}
	cleanupInfoJSON(targetDir)

	files, err := d.renameAllByMeta(targetDir)
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("playlist download produced no mp4 files in %s", targetDir)
	}
	return targetDir, files, nil
}

func (d *Downloader) retryMissingVideos(dir string, ids []string) error {
	if len(ids) == 0 {
		ids = videoIDsFromMP4(dir)
	}
	var missing []string
	for _, id := range ids {
		if len(id) != 11 {
			continue
		}
		if findNamedVideo(dir, id) != "" {
			continue
		}
		mp4Path := filepath.Join(dir, id+".mp4")
		if info, err := os.Stat(mp4Path); err == nil && info.Size() > 0 {
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) == 0 {
		return nil
	}
	fmt.Printf("retry missing videos: %d\n", len(missing))

	var failed []string
	for _, id := range missing {
		videoURL := "https://www.youtube.com/watch?v=" + id
		outputPath := filepath.Join(dir, id+".mp4")
		if err := d.downloadWithRetry(videoURL, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			failed = append(failed, id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d video(s) still missing: %v", len(failed), failed)
	}
	return nil
}

func (d *Downloader) renameAllByMeta(dir string) ([]string, error) {
	raw, err := listMP4(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	for _, path := range raw {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		videoURL := "https://www.youtube.com/watch?v=" + base
		if len(base) != 11 {
			// already renamed
			out = append(out, path)
			continue
		}
		named, err := d.renameByMeta(path, videoURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: rename %s: %v\n", path, err)
			out = append(out, path)
			continue
		}
		out = append(out, named)
	}
	cleanupInfoJSON(dir)
	return out, nil
}

func (d *Downloader) renameByMeta(videoPath, videoURL string) (string, error) {
	removeSidecars(videoPath)
	meta := d.ResolveTrackMeta(videoPath, videoURL)
	base := FormatFileName(meta)
	if base == "" || base == "unknown" {
		return videoPath, nil
	}
	dir := filepath.Dir(videoPath)
	target := uniquePath(dir, base, ".mp4")
	if filepath.Clean(target) == filepath.Clean(videoPath) {
		return videoPath, nil
	}
	if err := os.Rename(videoPath, target); err != nil {
		return "", err
	}
	fmt.Println("renamed:", filepath.Base(videoPath), "->", filepath.Base(target))
	return target, nil
}

// PlaylistVideoIDs lists video ids in a playlist (flat, no download).
func (d *Downloader) PlaylistVideoIDs(playlistURL string) ([]string, error) {
	cmd := exec.Command(d.YTDLP,
		"--flat-playlist",
		"--print", "%(id)s",
		playlistURL,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var ids []string
	for _, line := range strings.Split(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "\n") {
		id := strings.TrimSpace(line)
		if len(id) == 11 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// PlaylistTitle asks yt-dlp for the playlist title (and verifies the playlist is readable).
func (d *Downloader) PlaylistTitle(playlistURL string) (string, error) {
	cmd := exec.Command(d.YTDLP,
		"--flat-playlist",
		"-J",
		"--playlist-items", "1",
		playlistURL,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "403") || strings.Contains(msg, "does not have permission") {
			return "", fmt.Errorf("youtube returned 403 (private/restricted playlist?): %w: %s", err, msg)
		}
		if strings.Contains(msg, "Unable to recognize playlist") {
			return "", fmt.Errorf("yt-dlp could not recognize playlist: %w: %s", err, msg)
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}

	var payload struct {
		Title         string `json:"title"`
		PlaylistTitle string `json:"playlist_title"`
		Type          string `json:"_type"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return "", fmt.Errorf("parse playlist json: %w", err)
	}
	if payload.Type != "" && payload.Type != "playlist" {
		return "", fmt.Errorf("url resolved to %q, not a playlist", payload.Type)
	}
	title := firstNonEmpty(cleanMetaValue(payload.PlaylistTitle), cleanMetaValue(payload.Title))
	if title == "" {
		return "", fmt.Errorf("empty playlist title")
	}
	return title, nil
}

// NormalizeURL accepts a full URL or bare video id.
// When asPlaylist is true, rewrites to https://www.youtube.com/playlist?list=ID.
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
		if listID := PlaylistID(raw); listID != "" {
			return "https://www.youtube.com/playlist?list=" + listID
		}
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

func videoIDsFromMP4(dir string) []string {
	files, err := listMP4(dir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		if len(base) == 11 {
			ids = append(ids, base)
		}
	}
	return ids
}

func findNamedVideo(dir, videoID string) string {
	// Already-finalized files won't contain the id; only skip exact id.mp4 here.
	path := filepath.Join(dir, videoID+".mp4")
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path
	}
	return ""
}

func uniquePath(dir, base, ext string) string {
	candidate := filepath.Join(dir, base+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func removeSidecars(mediaPath string) {
	base := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	for _, p := range []string{base + ".info.json", base + ".description", base + ".jpg", base + ".webp"} {
		_ = os.Remove(p)
	}
}

func cleanupInfoJSON(dir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.info.json"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p)
	}
}
