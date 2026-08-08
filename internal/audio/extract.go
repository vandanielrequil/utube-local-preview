package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tags are optional ID3 fields written into the mp3.
type Tags struct {
	Artist string
	Title  string
	Album  string
	Year   string
}

// Extractor pulls audio tracks into mp3 files using system ffmpeg
// (same external-tool style as all-for-one-drive/internal/video-converter).
type Extractor struct {
	FFmpeg string
}

func NewExtractor() (*Extractor, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH")
	}
	return &Extractor{FFmpeg: ffmpeg}, nil
}

// ExtractMP3 writes outputDir/<baseName>.mp3 and fills ID3 tags when present.
// If baseName is empty, the source file base name is used.
// Existing non-empty mp3 files are skipped.
func (e *Extractor) ExtractMP3(sourcePath, outputDir, baseName string, tags Tags) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create music dir: %w", err)
	}

	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		baseName = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	outputPath := filepath.Join(outputDir, baseName+".mp3")
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		fmt.Println("skip existing mp3:", outputPath)
		return outputPath, nil
	}

	args := []string{
		"-loglevel", "info",
		"-stats",
		"-nostdin",
		"-y",
		"-i", sourcePath,
		"-vn",
		"-map_metadata", "-1",
		"-c:a", "libmp3lame",
		"-q:a", "2",
		"-id3v2_version", "3",
	}
	if tags.Title != "" {
		args = append(args, "-metadata", "title="+tags.Title)
	}
	if tags.Artist != "" {
		args = append(args, "-metadata", "artist="+tags.Artist)
	}
	if tags.Album != "" {
		args = append(args, "-metadata", "album="+tags.Album)
	}
	if tags.Year != "" {
		args = append(args, "-metadata", "date="+tags.Year)
	}
	args = append(args, outputPath)

	fmt.Fprintf(os.Stderr, "ffmpeg: %s -> %s\n", sourcePath, outputPath)
	fmt.Fprintf(os.Stderr, "tags: artist=%q title=%q album=%q year=%q\n", tags.Artist, tags.Title, tags.Album, tags.Year)

	cmd := exec.Command(e.FFmpeg, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg extract failed for %q: %w", sourcePath, err)
	}

	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("mp3 missing or empty: %s", outputPath)
	}
	return outputPath, nil
}
