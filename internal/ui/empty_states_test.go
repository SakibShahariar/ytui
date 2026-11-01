package ui

import (
	"strings"
	"testing"
)

func TestEmptyStateContentProvidesTabSpecificNextSteps(t *testing.T) {
	tests := []struct {
		name string
		model Model
		want []string
	}{
		{
			name: "search no results",
			model: Model{activeTab: tabSearch, everSearched: true},
			want: []string{"No videos found", "yt-dlp"},
		},
		{
			name: "search loading",
			model: Model{activeTab: tabSearch, loading: true},
			want: []string{"Searching YouTube", "search finishes"},
		},
		{
			name: "home loading",
			model: Model{activeTab: tabHome, ytLoading: true, loggedIn: true},
			want: []string{"Loading your home feed", "recommendations"},
		},
		{
			name: "subscriptions while signed out",
			model: Model{activeTab: tabSubscriptions},
			want: []string{"Google login required", "Ctrl+L"},
		},
		{
			name: "subscriptions empty",
			model: Model{activeTab: tabSubscriptions, loggedIn: true},
			want: []string{"No subscriptions yet", "Ctrl+S"},
		},
		{
			name: "playlists empty",
			model: Model{activeTab: tabPlaylists, loggedIn: true},
			want: []string{"No playlists found", "Google"},
		},
		{
			name: "liked empty",
			model: Model{activeTab: tabLiked, loggedIn: true},
			want: []string{"No liked videos found", "Google"},
		},
		{
			name: "watch later empty",
			model: Model{activeTab: tabWatchLater},
			want: []string{"Watch Later", "Ctrl+W"},
		},
		{
			name: "history empty",
			model: Model{activeTab: tabHistory},
			want: []string{"No watch history yet", "played through ytui"},
		},
		{
			name: "live downloads empty",
			model: Model{activeTab: tabDownloads},
			want: []string{"Download queue is empty", "Ctrl+D"},
		},
		{
			name: "offline downloads empty",
			model: Model{activeTab: tabDownloads, offlineDownloads: true},
			want: []string{"No offline files found", "Ctrl+D"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.emptyStateContent(80, 20)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("empty state %q does not contain %q", got, want)
				}
			}
		})
	}
}


func TestErrorRecoveryHintProvidesContextualNextSteps(t *testing.T) {
	tests := []struct { message, want string }{
		{"network connection timed out", "check your connection"},
		{"yt-dlp executable not found", "install or update yt-dlp"},
		{"OAuth token unauthorized", "Ctrl+L"},
		{"permission denied while writing download", "check permissions"},
		{"unexpected parser failure", ""},
	}
	for _, tt := range tests {
		if got := errorRecoveryHint(tt.message); !strings.Contains(got, tt.want) {
			t.Errorf("errorRecoveryHint(%q) = %q, want it to contain %q", tt.message, got, tt.want)
		}
	}
}
