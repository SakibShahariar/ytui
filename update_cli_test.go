package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCommandShowsLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v0.1.0","html_url":"https://example.com/release"}`))
	}))
	defer server.Close()
	original := updateEndpoint
	updateEndpoint = server.URL
	defer func() { updateEndpoint = original }()

	var out, errOut strings.Builder
	if err := runUpdateCommand([]string{"--check"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"current:", "latest:  v0.1.0", "release notes: https://example.com/release", "update available"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestUpdateCommandRejectsUnknownFlag(t *testing.T) {
	var out, errOut strings.Builder
	err := runUpdateCommand([]string{"--install"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("error = %v, want usage error", err)
	}
}
