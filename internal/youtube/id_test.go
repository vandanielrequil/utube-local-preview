package youtube

import (
	"strings"
	"testing"
)

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":     "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                     "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":      "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1": "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1": "OnzkhQsmSag",
	}
	for input, want := range cases {
		if got := VideoID(input); got != want {
			t.Fatalf("VideoID(%q)=%q want %q", input, got, want)
		}
	}
}

func TestNormalizeURLStripsPlaylist(t *testing.T) {
	got := NormalizeURL("https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1", false)
	want := "https://www.youtube.com/watch?v=OnzkhQsmSag"
	if got != want {
		t.Fatalf("NormalizeURL=%q want %q", got, want)
	}
}

func TestNormalizeURLKeepsPlaylist(t *testing.T) {
	raw := "https://www.youtube.com/watch?v=OnzkhQsmSag&list=RDOnzkhQsmSag&start_radio=1"
	got := NormalizeURL(raw, true)
	if got != raw {
		t.Fatalf("NormalizeURL playlist mode changed url: %q", got)
	}
}

func TestSanitizeDirName(t *testing.T) {
	got := SanitizeDirName(`Mix: Foo/Bar|Baz?*`)
	for _, bad := range `<>:"/\|?*` {
		if strings.ContainsRune(got, bad) {
			t.Fatalf("unsafe dir name: %q", got)
		}
	}
	if got == "" {
		t.Fatal("expected non-empty sanitized name")
	}
}
