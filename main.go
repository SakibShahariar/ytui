package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/store"
	"github.com/SakibShahariar/ytui/internal/ui"
)

// version may be set at build time with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func currentVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// confirmImport makes the destructive effect explicit and keeps the prompt
// independently testable without starting Bubble Tea.
func confirmImport(reader io.Reader, writer io.Writer, source, destination string) (bool, error) {
	fmt.Fprintf(writer, "Import %q and replace active settings at %q? [y/N] ", source, destination)
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit")
	exportPath := flag.String("export-config", "", "export effective settings to FILE and exit")
	importPath := flag.String("import-config", "", "import settings from FILE before starting ytui")
	yes := flag.Bool("yes", false, "confirm --import-config without prompting")
	showPaths := flag.Bool("show-paths", false, "print config and data paths and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("ytui %s\n", currentVersion())
		return
	}

	if *showPaths {
		dataPath, err := store.Path()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to resolve data path:", err)
			os.Exit(1)
		}
		fmt.Println("Config:", config.Path())
		fmt.Println("Data:", dataPath)
		return
	}

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
		if !*yes {
			confirmed, err := confirmImport(os.Stdin, os.Stderr, *importPath, config.Path())
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to read import confirmation:", err)
				os.Exit(1)
			}
			if !confirmed {
				fmt.Fprintln(os.Stderr, "Config import cancelled; active settings were not changed.")
				return
			}
		}
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
	finalModel, runErr := p.Run()
	if final, ok := finalModel.(ui.Model); ok {
		if cleanupErr := final.Cleanup(); cleanupErr != nil {
			fmt.Fprintln(os.Stderr, "cleanup warning:", cleanupErr)
		}
	} else {
		// Even if Bubble Tea cannot return its final model, release shared
		// resources created before the program started.
		if cleanupErr := m.Cleanup(); cleanupErr != nil {
			fmt.Fprintln(os.Stderr, "cleanup warning:", cleanupErr)
		}
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "error running ytui:", runErr)
		os.Exit(1)
	}
}
