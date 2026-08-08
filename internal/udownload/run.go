package udownload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"utube-local-preview/internal/applog"
	"utube-local-preview/internal/apppath"
	"utube-local-preview/internal/audio"
	"utube-local-preview/internal/links"
	"utube-local-preview/internal/youtube"
)

const (
	LinksFileName    = "u-links.txt"
	DownloadsDirName = "u-downloads"
	MusicDirName     = "music"
)

type Options struct {
	ExtractMusic bool
}

// Run downloads URLs from u-links.txt next to the binary into u-downloads.
// When ExtractMusic is true, also writes mp3 files under u-downloads/music
// (or u-downloads/music/<playlist>/ for PLAYLIST_ entries).
func Run(opts Options) (runErr error) {
	if err := applog.Init(); err != nil {
		return err
	}
	defer func() {
		if cerr := applog.Close(); cerr != nil && runErr == nil {
			runErr = cerr
		}
	}()

	baseDir, err := apppath.DirNextToExecutable()
	if err != nil {
		return err
	}

	linksPath := filepath.Join(baseDir, LinksFileName)
	downloadsDir := filepath.Join(baseDir, DownloadsDirName)

	entries, err := links.ReadFile(linksPath)
	if err != nil {
		return err
	}

	downloader, err := youtube.NewDownloader()
	if err != nil {
		return err
	}

	var extractor *audio.Extractor
	if opts.ExtractMusic {
		extractor, err = audio.NewExtractor()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(downloadsDir, 0o755); err != nil {
		return fmt.Errorf("create downloads dir: %w", err)
	}

	musicRoot := filepath.Join(downloadsDir, MusicDirName)
	var failed []string

	for i, entry := range entries {
		label := entry.URL
		if entry.Playlist {
			label = "PLAYLIST " + entry.URL
		}
		fmt.Printf("[%d/%d] %s\n", i+1, len(entries), label)

		if entry.Playlist {
			playlistDir, files, err := downloader.DownloadPlaylist(entry.URL, downloadsDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				failed = append(failed, entry.URL)
				continue
			}
			if extractor == nil {
				continue
			}
			musicDir := filepath.Join(musicRoot, filepath.Base(playlistDir))
			for _, videoPath := range files {
				if err := extractOne(downloader, extractor, videoPath, musicDir); err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
					failed = append(failed, videoPath+" (mp3)")
				}
			}
			continue
		}

		videoPath, err := downloader.Download(entry.URL, downloadsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			failed = append(failed, entry.URL)
			continue
		}

		if extractor == nil {
			continue
		}
		if err := extractOne(downloader, extractor, videoPath, musicRoot); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			failed = append(failed, entry.URL+" (mp3)")
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed %d item(s): %v", len(failed), failed)
	}
	fmt.Printf("done: %d item(s) -> %s\n", len(entries), downloadsDir)
	return nil
}

func extractOne(downloader *youtube.Downloader, extractor *audio.Extractor, videoPath, musicDir string) error {
	// Prefer URL from 11-char id filename; after rename use basename as title fallback only.
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoURL := ""
	if len(base) == 11 {
		videoURL = "https://www.youtube.com/watch?v=" + base
	}
	meta := downloader.ResolveTrackMeta(videoPath, videoURL)
	if videoURL == "" {
		// File already renamed; meta lookup by URL may fail — use filename parts.
		if meta.Title == "" && meta.Track == "" {
			if left, right, ok := splitName(base); ok {
				meta.Artist, meta.Title = left, right
			} else {
				meta.Title = base
			}
		}
	}

	artist, title := meta.ResolvedNames()
	baseName := youtube.FormatFileName(meta)
	fmt.Println("music name:", baseName)
	_, err := extractor.ExtractMP3(videoPath, musicDir, baseName, audio.Tags{
		Artist: artist,
		Title:  title,
		Album:  meta.Album,
		Year:   meta.Year,
	})
	return err
}

func splitName(name string) (string, string, bool) {
	parts := strings.SplitN(name, " - ", 2)
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
