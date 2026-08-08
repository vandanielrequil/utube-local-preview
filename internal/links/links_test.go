package links

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileSpacesAndNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "u-links.txt")
	content := "https://youtu.be/aaaaaaaaaaa\nhttps://youtu.be/bbbbbbbbbbb https://youtu.be/ccccccccccc\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d links, want 3: %#v", len(got), got)
	}
	for _, e := range got {
		if e.Playlist {
			t.Fatalf("unexpected playlist flag: %#v", e)
		}
	}
}

func TestReadFilePlaylistPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "u-links.txt")
	content := "PLAYLIST_https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1\nhttps://youtu.be/aaaaaaaaaaa\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %#v", len(got), got)
	}
	if !got[0].Playlist {
		t.Fatalf("first entry should be playlist: %#v", got[0])
	}
	if got[0].URL != "https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1" {
		t.Fatalf("unexpected playlist url: %q", got[0].URL)
	}
	if got[1].Playlist {
		t.Fatalf("second entry should be single video: %#v", got[1])
	}
}
