package library

import (
	"os"
	"path/filepath"
	"strings"

	id3v2 "github.com/bogem/id3v2/v2"
)

// Probe reads Title/Artist/Album tags best-effort. Never returns an error:
// on any failure all strings are empty and the caller falls back to filename.
func Probe(path string) (title, artist, album string) {
	if strings.ToLower(filepath.Ext(path)) != ".mp3" {
		return "", "", ""
	}
	f, err := os.Open(path)
	if err != nil {
		return "", "", ""
	}
	defer f.Close()
	tag, err := id3v2.ParseReader(f, id3v2.Options{Parse: true})
	if err != nil || tag == nil {
		return "", "", ""
	}
	defer tag.Close()
	return tag.GetTextFrame("TIT2").Text,
		tag.GetTextFrame("TPE1").Text,
		tag.GetTextFrame("TALB").Text
}
