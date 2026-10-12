package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanDirFiltersAudio(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.mp3", "b.FLAC", "c.wav", "d.ogg", "e.txt", "f.jpg", "g.M4A", "noext"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tracks, err := ScanDir(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 5 {
		t.Fatalf("want 5 audio tracks, got %d: %+v", len(tracks), tracks)
	}
	// sorted by name
	if tracks[0].Name != "a" || tracks[4].Name != "g" {
		t.Fatalf("bad sort: %+v", tracks)
	}
	if !IsSupported(".mp3") || IsSupported(".txt") {
		t.Fatal("IsSupported wrong")
	}
}

func TestScanDirMissing(t *testing.T) {
	if _, err := ScanDir(filepath.Join(t.TempDir(), "nope"), false); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[int64]string{
		0:    "--:--",
		-5:   "--:--",
		65:   "01:05",
		3661: "1:01:01",
	}
	for sec, want := range cases {
		got := FormatDuration(time.Duration(sec) * time.Second)
		if got != want {
			t.Errorf("sec=%d want %q got %q", sec, want, got)
		}
	}
}
