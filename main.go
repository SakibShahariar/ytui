package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	configOverride := flag.String("config", "", "use config file PATH instead of the XDG default")
	dataOverride := flag.String("data", "", "use data store file PATH instead of the XDG default")
	downloadDir := flag.String("dir", "", "save downloads and browse offline files in DIR")
	flag.Parse()

	if showVersion {
		fmt.Printf("ytui %s\n", currentVersion())
		return
	}

	if *downloadDir != "" {
		resolved, err := prepareDownloadDir(*downloadDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid --dir:", err)
			os.Exit(2)
		}
		if err := os.Setenv("YTUI_DOWNLOAD_DIR", resolved); err != nil {
			fmt.Fprintln(os.Stderr, "invalid --dir:", err)
			os.Exit(2)
		}
	}

	if *configOverride != "" {
		if err := os.Setenv("YTUI_CONFIG_PATH", *configOverride); err != nil {
			fmt.Fprintln(os.Stderr, "invalid --config path:", err)
			os.Exit(2)
		}
	}
	if *dataOverride != "" {
		if err := os.Setenv("YTUI_DATA_PATH", *dataOverride); err != nil {
			fmt.Fprintln(os.Stderr, "invalid --data path:", err)
			os.Exit(2)
		}
	}

	if handled, err := runSubcommand(flag.Args(), os.Stdin, os.Stdout, os.Stderr, *yes); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "ytui:", err)
			os.Exit(2)
		}
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

	var directURL string
	if len(flag.Args()) > 0 {
		if len(flag.Args()) != 1 {
			fmt.Fprintln(os.Stderr, "expected a single video URL argument")
			os.Exit(2)
		}
		var err error
		directURL, err = validateVideoURL(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid video URL:", err)
			os.Exit(2)
		}
	}

	s, err := store.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load local data:", err)
		os.Exit(1)
	}

	m := ui.New(s)
	if directURL != "" {
		m = ui.NewWithURL(s, directURL)
	}
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


// prepareDownloadDir validates a CLI download root and creates it if needed.
func prepareDownloadDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("directory path must not be empty")
	}
	for _, r := range path {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("directory path must not contain control characters")
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", abs)
	}
	return filepath.Clean(abs), nil
}
