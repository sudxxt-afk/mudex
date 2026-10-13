// Command player: neon TUI music player with folder browser. Usage: player [dir]
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"tui-player/internal/fonts"
	"tui-player/internal/ui"
)

func main() {
	installFont := flag.Bool("install-font", false, "install Nerd Font and exit")
	flag.Parse()
	if *installFont {
		fmt.Println(fonts.Ensure())
		return
	}
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "not a directory: %s\n", dir)
		os.Exit(1)
	}
	m := ui.New(dir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui:", err)
		os.Exit(1)
	}
}
