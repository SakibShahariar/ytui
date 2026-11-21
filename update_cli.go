package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/SakibShahariar/ytui/internal/update"
)

var updateEndpoint = update.DefaultEndpoint

func runUpdateCommand(args []string, stdout, stderr io.Writer) error {
	for _, arg := range args {
		if arg != "--check" {
			return &exitError{code: 2, err: fmt.Errorf("usage: ytui update [--check]")}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := update.Check(ctx, nil, updateEndpoint)
	if errors.Is(err, update.ErrNoReleases) {
		fmt.Fprintln(stdout, "no releases published yet")
		return nil
	}
	if err != nil {
		return &exitError{code: 1, err: fmt.Errorf("update check failed: %w", err)}
	}
	current := currentVersion()
	fmt.Fprintf(stdout, "current: %s\nlatest:  %s\n", current, release.TagName)
	if !release.PublishedAt.IsZero() {
		fmt.Fprintf(stdout, "published: %s\n", release.PublishedAt.Format("2006-01-02"))
	}
	if release.HTMLURL != "" {
		fmt.Fprintf(stdout, "release notes: %s\n", release.HTMLURL)
	}
	if update.NewerThan(current, release.TagName) {
		fmt.Fprintln(stdout, "update available (ytui never installs updates automatically)")
	} else {
		fmt.Fprintln(stdout, "up to date")
	}
	return nil
}
