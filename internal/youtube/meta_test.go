package youtube

import "testing"

func TestFormatFileName(t *testing.T) {
	cases := []struct {
		meta TrackMeta
		want string
	}{
		{
			meta: TrackMeta{Artist: "Daft Punk", Track: "Get Lucky"},
			want: "Daft Punk - Get Lucky",
		},
		{
			meta: TrackMeta{Title: "Daft Punk - Get Lucky"},
			want: "Daft Punk - Get Lucky",
		},
		{
			meta: TrackMeta{Artist: "Daft Punk", Title: "Daft Punk - Get Lucky"},
			want: "Daft Punk - Get Lucky",
		},
		{
			meta: TrackMeta{Title: "Get Lucky", Source: "DaftPunkVEVO"},
			want: "DaftPunkVEVO - Get Lucky",
		},
		{
			meta: TrackMeta{Title: "Just A Title"},
			want: "Just A Title",
		},
		{
			meta: TrackMeta{Artist: "AC/DC", Track: "Back In Black?"},
			want: "AC DC - Back In Black",
		},
	}
	for _, tc := range cases {
		if got := FormatFileName(tc.meta); got != tc.want {
			t.Fatalf("FormatFileName(%+v)=%q want %q", tc.meta, got, tc.want)
		}
	}
}
