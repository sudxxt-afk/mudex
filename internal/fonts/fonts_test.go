package fonts

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestPickFontFileExact(t *testing.T) {
	names := []string{"a/JetBrainsMono-Bold.ttf", "b/" + wantFile, "c/OtherMono-Regular.ttf"}
	if got := pickFontFile(names); got != "b/"+wantFile {
		t.Fatalf("want exact match, got %q", got)
	}
}

func TestPickFontFileFallback(t *testing.T) {
	names := []string{"a/JetBrainsMono-Bold.ttf", "c/OtherMono-Regular.ttf"}
	if got := pickFontFile(names); got != "c/OtherMono-Regular.ttf" {
		t.Fatalf("want mono-regular fallback, got %q", got)
	}
	if pickFontFile([]string{"a/Bold.ttf"}) != "" {
		t.Fatal("want empty when nothing matches")
	}
}

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.zip")
	w, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for name, body := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractHappy(t *testing.T) {
	zp := makeZip(t, map[string]string{
		"a/JetBrainsMono-Bold.ttf": "bold",
		"b/" + wantFile:            "regular",
	})
	dest := t.TempDir()
	name, err := extract(zp, dest)
	if err != nil {
		t.Fatal(err)
	}
	if name != wantFile {
		t.Fatalf("want %s got %s", wantFile, name)
	}
	body, err := os.ReadFile(filepath.Join(dest, wantFile))
	if err != nil || string(body) != "regular" {
		t.Fatalf("bad extracted content: %q %v", body, err)
	}
}

func TestExtractNoMatch(t *testing.T) {
	zp := makeZip(t, map[string]string{"a/Bold.ttf": "x"})
	if _, err := extract(zp, t.TempDir()); err == nil {
		t.Fatal("want error when no mono font inside")
	}
}

func TestInstalledNoCrash(t *testing.T) {
	_ = Installed() // environment-dependent, must only not crash
}
