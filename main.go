package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ui"
)

func main() {
	exportPath := flag.String("export-config", "", "export effective settings to FILE and exit")
	importPath := flag.String("import-config", "", "import settings from FILE before starting ytui")
	flag.Parse()

	if *exportPath != "" && *importPath != "" {
		fmt.Fprintln(os.Stderr, "use either --export-config or --import-config, not both")
		os.Exit(2)
	}
	if *exportPath != "" {
		if err := config.Export(*exportPath); err != nil {
			fmt.Fprintln(os.Stderr, "failed to export config:", err)
			os.Exit(1)
		}
		fmt.Println("Exported settings to", *exportPath)
		return
	}
	if *importPath != "" {
		if err := config.Import(*importPath); err != nil {
			fmt.Fprintln(os.Stderr, "failed to import config:", err)
			os.Exit(1)
		}
		fmt.Println("Imported settings from", *importPath)
	}

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
