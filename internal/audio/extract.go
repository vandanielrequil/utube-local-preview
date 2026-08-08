package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// ExtractMP3 writes outputDir/<baseName>.mp3 from a video/audio source.
// If baseName is empty, the source file base name is used.
// Existing non-empty mp3 files are skipped.
func (e *Extractor) ExtractMP3(sourcePath, outputDir, baseName string) (string, error) {
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
		"-c:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	}

	fmt.Fprintf(os.Stderr, "ffmpeg: %s -> %s\n", sourcePath, outputPath)
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
