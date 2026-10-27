package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/SakibShahariar/ytui/internal/auth"
	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/store"
)

type doctorTool struct {
	name    string
	path    string
	version string
	status  string
}

type doctorReport struct {
	version       string
	configPath    string
	configStatus  string
	dataPath      string
	dataStatus    string
	authStatus    string
	downloadPath  string
	downloadState string
	tools         []doctorTool
}

func runDoctor(w io.Writer) error {
	return renderDoctorReport(w, collectDoctorReport())
}

func collectDoctorReport() doctorReport {
	cfg, cfgErr := config.LoadWithError()
	configPath := config.Path()
	configStatus := "valid"
	if cfgErr != nil {
		configStatus = "invalid/unreadable: " + cleanDiagnostic(cfgErr.Error())
	} else if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configStatus = "not created; built-in defaults active"
	} else if err != nil {
		configStatus = "unavailable: " + cleanDiagnostic(err.Error())
	}

	dataPath, dataErr := store.Path()
	dataStatus := "unavailable"
	if dataErr != nil {
		dataStatus = cleanDiagnostic(dataErr.Error())
	} else {
		dataStatus = pathStatus(dataPath, false)
	}

	authStatus := "not configured"
	if auth.HaveToken() {
		authStatus = "token present (contents not read or displayed)"
	}

	home, _ := os.UserHomeDir()
	downloadPath := cfg.DownloadDir
	if downloadPath == "" && home != "" {
		downloadPath = filepath.Join(home, "Downloads", "ytui")
	}
	downloadState := "unavailable"
	if downloadPath != "" {
		downloadState = pathStatus(downloadPath, true)
	}

	tools := make([]doctorTool, 0, 2)
	for _, name := range []string{"mpv", "yt-dlp"} {
		path, err := exec.LookPath(name)
		if err != nil {
			tools = append(tools, doctorTool{name: name, status: "missing from PATH"})
			continue
		}
		version, versionErr := toolVersion(path)
		status := "available"
		if versionErr != nil {
			status = "available; version check failed: " + cleanDiagnostic(versionErr.Error())
		}
		tools = append(tools, doctorTool{name: name, path: path, version: version, status: status})
	}

	return doctorReport{
		version: currentVersion(), configPath: configPath, configStatus: configStatus,
		dataPath: dataPath, dataStatus: dataStatus, authStatus: authStatus,
		downloadPath: downloadPath, downloadState: downloadState, tools: tools,
	}
}

func pathStatus(path string, directory bool) string {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "not created yet"
	}
	if err != nil {
		return "unavailable: " + cleanDiagnostic(err.Error())
	}
	if directory && !info.IsDir() {
		return "error: path exists but is not a directory"
	}
	if directory {
		return "directory exists"
	}
	if info.IsDir() {
		return "error: expected a file but found a directory"
	}
	return fmt.Sprintf("file exists (%d bytes)", info.Size())
}

func toolVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(output), "\n", 2)[0]
	return cleanDiagnostic(line), nil
}

func cleanDiagnostic(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 160 {
		value = string(runes[:160]) + "…"
	}
	return value
}

func renderDoctorReport(w io.Writer, report doctorReport) error {
	if _, err := fmt.Fprintf(w, "ytui doctor\nVersion: %s\n", cleanDiagnostic(report.version)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Config: %s (%s)\n", cleanDiagnostic(report.configPath), cleanDiagnostic(report.configStatus)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Data: %s (%s)\n", cleanDiagnostic(report.dataPath), cleanDiagnostic(report.dataStatus)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Authentication: %s\n", cleanDiagnostic(report.authStatus)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Downloads: %s (%s)\n", cleanDiagnostic(report.downloadPath), cleanDiagnostic(report.downloadState)); err != nil {
		return err
	}
	for _, tool := range report.tools {
		detail := cleanDiagnostic(tool.status)
		if tool.path != "" {
			detail += "; path: " + cleanDiagnostic(tool.path)
		}
		if tool.version != "" {
			detail += "; version: " + cleanDiagnostic(tool.version)
		}
		if _, err := fmt.Fprintf(w, "%s: %s\n", cleanDiagnostic(tool.name), detail); err != nil {
			return err
		}
	}
	return nil
}
