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
				baseName := musicBaseName(downloader, videoURLFromDownloaded(videoPath))
				if _, err := extractor.ExtractMP3(videoPath, musicDir, baseName); err != nil {
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
		baseName := musicBaseName(downloader, entry.URL)
		if _, err := extractor.ExtractMP3(videoPath, musicRoot, baseName); err != nil {
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

func musicBaseName(downloader *youtube.Downloader, videoURL string) string {
	meta, err := downloader.TrackMetaForURL(videoURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: metadata: %v\n", err)
		return ""
	}
	name := youtube.FormatMusicFileName(meta)
	fmt.Println("music name:", name)
	return name
}

func videoURLFromDownloaded(videoPath string) string {
	id := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	return "https://www.youtube.com/watch?v=" + id
}
