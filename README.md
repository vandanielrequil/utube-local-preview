# utube-local-preview

Download YouTube media locally for offline preview.

## Binaries

| Binary | Behavior |
|--------|----------|
| `u-downloader` | Reads `u-links.txt` next to the exe, creates `u-downloads/`, downloads videos there |
| `u-downloader-music` | Same, then extracts audio to `u-downloads/music/*.mp3` via `ffmpeg` |

Place `u-links.txt` next to the binary. Links may be separated by spaces or newlines.

Prefix a URL with `PLAYLIST_` to download the whole playlist/mix into
`u-downloads/<playlist title>/` (or today's date if the title is unavailable).
Without the prefix, only the single video is downloaded (playlist params are ignored).

```text
https://www.youtube.com/watch?v=aaaaaaaaaaa
https://youtu.be/bbbbbbbbbbb https://youtu.be/ccccccccccc
PLAYLIST_https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1
PLAYLIST_https://www.youtube.com/playlist?list=PLxxxxxxxx
```

Music mode mirrors that layout under `u-downloads/music/` (and `u-downloads/music/<playlist>/`).
MP3 files are named `Artist - Track.mp3` from yt-dlp metadata.

Each run writes a fresh log next to the binary: `u-downloader.log` / `u-downloader-music.log`
(all Go + yt-dlp/ffmpeg output, truncated on start).

## Requirements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) on `PATH` (same as in `instab`)
- [ffmpeg](https://www.gyan.dev/ffmpeg/builds/) on `PATH` for `u-downloader-music` (and for yt-dlp merge). Windows: `winget install Gyan.FFmpeg`

## Build

```bash
go build -o u-downloader.exe ./cmd/u-downloader
go build -o u-downloader-music.exe ./cmd/u-downloader-music
```

## Layout

```text
cmd/
  u-downloader/
  u-downloader-music/
internal/
  youtube/     # yt-dlp download + track metadata naming
  audio/       # ffmpeg mp3 extract
  links/       # parse u-links.txt (PLAYLIST_ prefix)
  apppath/     # paths next to the binary
  applog/      # tee stdout/stderr into <exe>.log (truncate each run)
  udownload/   # shared CLI flow
```
