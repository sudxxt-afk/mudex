// Package library scans a directory for audio files and reads basic metadata.
package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Supported extensions (lowercase, with dot).
var SupportedExt = map[string]bool{
	".mp3":  true,
	".flac": true,
	".wav":  true,
	".ogg":  true,
	".oga":  true,
	".opus": true,
	".m4a":  true,
	".aac":  true,
	".wma":  true,
}

// Track is a single audio file entry.
type Track struct {
	Path     string
	Name     string // filename without extension
	Title    string // from tags, may be empty
	Artist   string // from tags, may be empty
	Album    string // from tags, may be empty
	Ext      string
	Size     int64
	ModTime  time.Time
	Duration time.Duration // may be 0 if unknown
	HasCover bool          // filled by cover probe (best effort)
}

// DisplayTitle returns the tag title or falls back to filename.
func (t Track) DisplayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	return t.Name
}

// DisplayArtist returns " — artist" suffix or "" when unknown.
func (t Track) DisplayArtist() string {
	if t.Artist != "" {
		return " — " + t.Artist
	}
	return ""
}

// IsSupported reports whether ext (with or without dot) is a known audio ext.
func IsSupported(ext string) bool {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return SupportedExt[ext]
}

// ScanDir lists supported audio files in dir (non-recursive by default).
// If recursive is true it walks subdirectories.
func ScanDir(dir string, recursive bool) ([]Track, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "scan", Path: dir, Err: os.ErrInvalid}
	}

	var out []Track
	if recursive {
		err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if t, ok := trackFromPath(p, d); ok {
				out = append(out, t)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if t, ok := trackFromPath(filepath.Join(dir, e.Name()), e); ok {
				out = append(out, t)
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func trackFromPath(full string, e os.DirEntry) (Track, bool) {
	ext := strings.ToLower(filepath.Ext(full))
	if !SupportedExt[ext] {
		return Track{}, false
	}
	fi, err := e.Info()
	if err != nil {
		return Track{}, false
	}
	title, artist, album := Probe(full)
	return Track{
		Path:     full,
		Name:     strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
		Title:    title,
		Artist:   artist,
		Album:    album,
		Ext:      ext,
		Size:     fi.Size(),
		ModTime:  fi.ModTime(),
		Duration: ProbeDuration(full),
	}, true
}

// FormatDuration renders d as m:ss or h:mm:ss.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "--:--"
	}
	s := int(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return intToStr(h) + ":" + itoa2(m) + ":" + itoa2(sec)
	}
	return itoa2(m) + ":" + itoa2(sec)
}

func itoa2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	if n < 100 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return strings.TrimSpace(strings.Repeat(" ", 0) + intToStr(n))
}

func intToStr(n int) string {
	if n == 0 {
		return "00"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
