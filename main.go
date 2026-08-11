package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ytui/internal/store"
	"ytui/internal/ui"
)

func main() {
	s, err := store.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load local data:", err)
		os.Exit(1)
	}

	m := ui.New(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running ytui:", err)
		os.Exit(1)
	}
}
