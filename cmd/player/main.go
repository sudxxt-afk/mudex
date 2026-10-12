// Command player: neon TUI music player. Usage: player [dir] [-r]
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"tui-player/internal/library"
	"tui-player/internal/ui"
)

func main() {
	recursive := flag.Bool("r", false, "scan subdirectories recursively")
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "not a directory: %s\n", dir)
		os.Exit(1)
	}
	tracks, err := library.ScanDir(dir, *recursive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(1)
	}
	if len(tracks) == 0 {
		fmt.Fprintf(os.Stderr, "no audio files in %s (mp3/flac/wav/ogg/opus/m4a/aac/wma)\n", dir)
		os.Exit(1)
	}
	m := ui.New(dir, tracks)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}
