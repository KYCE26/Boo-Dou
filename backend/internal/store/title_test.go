package store

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTitleFrom(t *testing.T) {
	long := "Tolong jelaskan perbedaan goroutine dan thread sistem operasi secara singkat dan jelas"
	cases := []struct {
		name, in string
		images   bool
		want     string
	}{
		{"pendek", "Halo Boo Dou", false, "Halo Boo Dou"},
		{"spasi dan baris baru dirapikan", "  Halo\n\n  apa   kabar ", false, "Halo apa kabar"},
		{"dipotong di batas kata", long, false, "Tolong jelaskan perbedaan goroutine dan thread sistem"},
		{"hanya gambar", "", true, "Gambar"},
		{"kosong", "   ", false, "Percakapan baru"},
	}
	for _, tc := range cases {
		if got := TitleFrom(tc.in, tc.images); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTitleFromNeverSplitsWordsOrRunes(t *testing.T) {
	in := strings.Repeat("kata ", 30) + "ééé"
	got := TitleFrom(in, false)
	if utf8.RuneCountInString(got) > titleMaxRunes || strings.HasSuffix(got, " ") || !strings.HasSuffix(got, "kata") {
		t.Fatalf("got %q", got)
	}
	// One giant word has no boundary to cut at; it must still be valid UTF-8.
	got = TitleFrom(strings.Repeat("é", 100), false)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != titleMaxRunes {
		t.Fatalf("got %q", got)
	}
}
