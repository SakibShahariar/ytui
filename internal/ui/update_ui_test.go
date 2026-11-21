package ui

import (
	"strings"
	"testing"

	"github.com/SakibShahariar/ytui/internal/config"
	"github.com/SakibShahariar/ytui/internal/update"
)

func TestUpdateCheckStatus(t *testing.T) {
	SetVersion("v0.0.1")
	m := Model{}
	updated, _ := m.handleUpdateCheck(updateCheckMsg{release: update.Release{
		TagName: "v0.1.0", HTMLURL: "https://example.com/release",
	}})
	status := updated.(Model).statusMsg
	if !strings.Contains(status, "v0.1.0") || !strings.Contains(status, "no automatic install") {
		t.Fatalf("status = %q", status)
	}
}

func TestAutoUpdateRespectsConfig(t *testing.T) {
	m := Model{cfg: configForUpdateTest(false, 0)}
	if cmd := m.maybeAutoUpdateCmd(); cmd != nil {
		t.Fatal("automatic check scheduled while disabled")
	}
}

func configForUpdateTest(enabled bool, hours int) (cfg config.Config) {
	cfg.UpdateCheckEnabled = enabled
	cfg.UpdateCheckIntervalHours = hours
	return
}
