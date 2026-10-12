// Package fonts ensures a Nerd Font is available for icons/symbols.
// On first run (in background) it downloads JetBrainsMono Nerd Font into
// ~/.local/share/fonts/mudex when no Nerd Font is registered with fontconfig.
// Every step is best-effort: the player fully works without it.
package fonts

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Pinned release (verified asset list); bump deliberately, not blindly.
	fontURL  = "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.5.1/JetBrainsMono.zip"
	wantFile = "JetBrainsMonoNerdFontMono-Regular.ttf"
	marker   = ".mudex-font-ok"
)

// Dir returns the user font directory we install into.
func Dir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "fonts", "mudex")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "fonts", "mudex")
}

// Installed reports whether any Nerd Font is registered with fontconfig,
// or we previously installed one (marker).
func Installed() bool {
	if dir := Dir(); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	out, err := exec.Command("fc-list", ":", "family").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(strings.ToLower(line), "nerd") {
			return true
		}
	}
	return false
}

// Ensure installs the font when missing. Returns a human-readable status.
func Ensure() string {
	if Installed() {
		return ""
	}
	dir := Dir()
	if dir == "" {
		return "font: no home dir, skipped"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "font: " + err.Error()
	}
	tmp, err := download(fontURL)
	if err != nil {
		return "font: download skipped (" + err.Error() + ")"
	}
	defer os.Remove(tmp)
	name, err := extract(tmp, dir)
	if err != nil {
		return "font: " + err.Error()
	}
	if out, err := exec.Command("fc-cache", "-f", dir).CombinedOutput(); err != nil {
		return fmt.Sprintf("font: extracted %s but fc-cache failed: %s", name, strings.TrimSpace(string(out)))
	}
	_ = os.WriteFile(filepath.Join(dir, marker), []byte(name+"\n"), 0o644)
	return "font: installed " + name + " — restart terminal for icons"
}

func download(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http %s", resp.Status)
	}
	if resp.ContentLength > 500<<20 {
		return "", fmt.Errorf("archive too large")
	}
	f, err := os.CreateTemp("", "mudex-font-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// pickFontFile chooses the file to extract: exact match first,
// then any *Mono-Regular.ttf fallback.
func pickFontFile(names []string) string {
	for _, n := range names {
		if filepath.Base(n) == wantFile {
			return n
		}
	}
	for _, n := range names {
		base := filepath.Base(n)
		if strings.HasSuffix(base, "Mono-Regular.ttf") {
			return n
		}
	}
	return ""
}

func extract(zipPath, destDir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	want := pickFontFile(names)
	if want == "" {
		return "", fmt.Errorf("no mono font in archive")
	}
	var src io.ReadCloser
	for _, f := range zr.File {
		if f.Name == want {
			src, err = f.Open()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if src == nil {
		return "", fmt.Errorf("archive entry vanished")
	}
	defer src.Close()
	dst, err := os.OpenFile(filepath.Join(destDir, filepath.Base(want)),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return filepath.Base(want), nil
}
