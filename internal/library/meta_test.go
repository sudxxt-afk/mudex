package library

import (
	"os"
	"path/filepath"
	"testing"

	id3v2 "github.com/bogem/id3v2/v2"
)

func writeTaggedMP3(t *testing.T, path, title, artist, album string) {
	t.Helper()
	// minimal file: empty body + ID3v2 tag written by the lib itself
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	tag, err := id3v2.Open(path, id3v2.Options{Parse: false})
	if err != nil {
		t.Fatal(err)
	}
	defer tag.Close()
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	tag.AddTextFrame("TIT2", tag.DefaultEncoding(), title)
	tag.AddTextFrame("TPE1", tag.DefaultEncoding(), artist)
	tag.AddTextFrame("TALB", tag.DefaultEncoding(), album)
	if err := tag.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestProbeTaggedMP3(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.mp3")
	writeTaggedMP3(t, p, "Neon Sky", "DJ Test", "Demo LP")
	title, artist, album := Probe(p)
	if title != "Neon Sky" || artist != "DJ Test" || album != "Demo LP" {
		t.Fatalf("got %q %q %q", title, artist, album)
	}
}

func TestProbeUntaggedFallsBackEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plain.mp3")
	if err := os.WriteFile(p, []byte("not really audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if a, b, c := Probe(p); a != "" || b != "" || c != "" {
		t.Fatalf("want empty, got %q %q %q", a, b, c)
	}
	if a, b, c := Probe(filepath.Join(dir, "x.wav")); a != "" || b != "" || c != "" {
		t.Fatalf("non-mp3 must be empty, got %q %q %q", a, b, c)
	}
	if a, b, c := Probe(filepath.Join(dir, "missing.mp3")); a != "" || b != "" || c != "" {
		t.Fatal("missing file must be empty")
	}
}

func TestDisplayFallback(t *testing.T) {
	tr := Track{Name: "song1"}
	if tr.DisplayTitle() != "song1" || tr.DisplayArtist() != "" {
		t.Fatal("fallback wrong")
	}
	tr.Title, tr.Artist = "Sky", "DJ"
	if tr.DisplayTitle() != "Sky" || tr.DisplayArtist() != " — DJ" {
		t.Fatal("tag display wrong")
	}
}
