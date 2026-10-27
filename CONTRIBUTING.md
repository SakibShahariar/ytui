# Contributing to ytui

Thanks for helping improve ytui.

## Development requirements

- Go 1.22 or newer
- [mpv](https://mpv.io/) for playback
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for YouTube search and downloads

## Build and test

Run these commands from the repository root:

```bash
go build -o ytui .
go test ./...
go vet ./...
```

The tests should not require an interactive GNOME/desktop session. Playback and download features may require mpv or yt-dlp when manually testing the full application.

## Changes and pull requests

- Keep changes focused and explain the user-facing reason.
- Add or update tests for behavior changes where practical.
- Update the README or documentation when commands, configuration, or keybindings change.
- Describe what you tested and include relevant, redacted error output when reporting a problem.

## Secrets and private data

Never commit OAuth tokens, client secrets, cookies, access tokens, personal configuration, or other private data. Redact credentials and personal information from logs before sharing them. Do not attach a full configuration or token file to an issue.
