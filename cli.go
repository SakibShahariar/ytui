package main

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/store"
)

func runSubcommand(args []string, stdin io.Reader, stdout, stderr io.Writer, assumeYes bool) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "help":
		_, err := fmt.Fprintln(stdout, `Usage: ytui [flags] [VIDEO_URL]
       ytui https://www.youtube.com/watch?v=VIDEO_ID

Commands:
  help                         Show this command reference
  version                      Print the installed version
  paths                        Print the effective config and data paths
  doctor                       Check dependencies and local state
  config export PATH           Export effective settings
  config import PATH [--yes]   Import settings with confirmation

Legacy flags remain supported:
  --version, --show-paths, --export-config FILE, --import-config FILE
  --config PATH, --data PATH, --yes`)
		return true, err
	case "version":
		if len(args) != 1 {
			return true, fmt.Errorf("usage: ytui version")
		}
		_, err := fmt.Fprintf(stdout, "ytui %s\n", currentVersion())
		return true, err
	case "paths":
		if len(args) != 1 {
			return true, fmt.Errorf("usage: ytui paths")
		}
		dataPath, err := store.Path()
		if err != nil {
			return true, fmt.Errorf("resolve data path: %w", err)
		}
		if _, err := fmt.Fprintln(stdout, "Config:", config.Path()); err != nil {
			return true, err
		}
		_, err = fmt.Fprintln(stdout, "Data:", dataPath)
		return true, err
	case "doctor":
		if len(args) != 1 {
			return true, fmt.Errorf("usage: ytui doctor")
		}
		return true, runDoctor(stdout)
	case "config":
		return true, runConfigSubcommand(args[1:], stdin, stdout, assumeYes)
	default:
		return false, nil
	}
}

func runConfigSubcommand(args []string, stdin io.Reader, stdout io.Writer, assumeYes bool) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprintln(stdout, `Usage:
  ytui config export PATH
  ytui config import PATH [--yes]`)
		return err
	}
	switch args[0] {
	case "export":
		if len(args) != 2 {
			return fmt.Errorf("usage: ytui config export PATH")
		}
		if err := config.Export(args[1]); err != nil {
			return fmt.Errorf("export config: %w", err)
		}
		_, err := fmt.Fprintln(stdout, "Exported settings to", args[1])
		return err
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: ytui config import PATH [--yes]")
		}
		yes := assumeYes
		for _, arg := range args[2:] {
			if arg != "--yes" {
				return fmt.Errorf("unexpected argument %q; use ytui config import PATH [--yes]", arg)
			}
			yes = true
		}
		if !yes {
			confirmed, err := confirmImport(stdin, stdout, args[1], config.Path())
			if err != nil {
				return fmt.Errorf("read import confirmation: %w", err)
			}
			if !confirmed {
				_, err := fmt.Fprintln(stdout, "Import canceled.")
				return err
			}
		}
		if err := config.Import(args[1]); err != nil {
			return fmt.Errorf("import config: %w", err)
		}
		_, err := fmt.Fprintln(stdout, "Imported settings from", args[1])
		return err
	default:
		return fmt.Errorf("unknown config command %q; use ytui config help", args[0])
	}
}

func validateVideoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("URL is empty")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", fmt.Errorf("enter a complete http(s) URL")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("only absolute http:// or https:// URLs are supported")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("URLs containing embedded credentials are not supported")
	}
	return parsed.String(), nil
}
