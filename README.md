<div align="center">

# ytui

**A keyboard-first YouTube client for your terminal.**

Search and play with mpv, manage your library, and queue downloads — without opening a browser.

[![Go tests](https://github.com/SakibShahariar/ytui/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/SakibShahariar/ytui/actions/workflows/tests.yml)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

## Features

- **Search and playback** — YouTube search, playlist browsing, direct URLs, and mpv playback.
- **Personal library** — subscriptions, Watch Later, history, and offline downloads.
- **Download queue** — persistent queue, retries, retained partial downloads, and desktop notifications.
- **Playback tools** — autoplay, subtitle selection, transcripts, and optional [SponsorBlock](https://wiki.sponsor.ajay.app/).
- **Google integration** — read-only YouTube access with isolated profiles for multiple accounts.
- **Terminal UI** — keyboard-driven navigation, command palette, and optional Kitty-compatible thumbnails.
- **Ytrawl integration** — import media URLs through a shared, permission-restricted queue.
- **Diagnostics** — `ytui doctor` checks dependencies and local state without printing credentials.

## Get started

### 1. Install dependencies

You need **Go 1.24+** to install or build from source, plus [mpv](https://mpv.io/) and [yt-dlp](https://github.com/yt-dlp/yt-dlp) for playback and media handling.

On Fedora:

```bash
sudo dnf install golang mpv yt-dlp
```

### 2. Install ytui

```bash
go install github.com/SakibShahariar/ytui@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`, then launch:

```bash
ytui
```

Or build from source:

```bash
git clone https://github.com/SakibShahariar/ytui.git
cd ytui
go build -o ytui .
./ytui
```

Tagged releases provide Linux and macOS archives for `amd64` and `arm64`. See [Releases](https://github.com/SakibShahariar/ytui/releases) and verify downloads with the included `SHA256SUMS`. Release binaries still require mpv and yt-dlp separately.

## Essential keys

| Key | Action |
|---|---|
| `Enter` | Search or play the selected item |
| `↑` / `↓` | Navigate lists |
| `Tab` / `Shift+Tab` | Switch tabs |
| `/` | Jump to Search |
| `:` | Open the command palette |
| `?` | Show keyboard help |
| `Ctrl+D` | Download options |
| `Ctrl+P` | Pause or resume playback |
| `Ctrl+X` | Stop playback |
| `Ctrl+L` | Google login / logout |
| `Ctrl+B` | SponsorBlock settings |
| `Ctrl+J` | Skip the active SponsorBlock segment |
| `Ctrl+V` | Choose subtitles |
| `Ctrl+E` | Open transcript, when available |
| `Ctrl+C` | Quit |

Paste a YouTube playlist URL into Search to browse its videos. Press `Ctrl+W` to add a video to Watch Later. On that tab, `s` cycles sorting, `Space` selects entries, `m` removes selected entries after confirmation, and `x` then `y` clears the queue.

See the [complete keybinding guide](docs/keybindings.md) for queue, history, download, and playback controls.

## Google accounts

ytui works without signing in. To access your YouTube subscriptions, playlists, and liked videos, press `Ctrl+L` and follow the [Google login setup](docs/google-login.md).

Multiple accounts can be managed from the CLI:

```bash
ytui profile list
ytui profile create work
ytui profile use work
# Launch ytui and press Ctrl+L to sign in
ytui profile use default
ytui profile remove work
```

Each profile has a separate token. OAuth app credentials are shared; token files are stored with owner-only permissions.

## Configuration and data

Settings are normally stored in `~/.config/ytui/config.json`; cache and application data use XDG-compatible directories. Change the download directory with `download_dir` or pass `--dir PATH` for a one-run override.

```bash
ytui --version
ytui doctor
ytui --show-paths
ytui config export ~/ytui-settings.json
ytui config import ~/ytui-settings.json
```

Config exports contain preferences only — not OAuth tokens, credentials, subscriptions, history, or Watch Later entries. For troubleshooting, logging is off by default; enable it with `--log-level error`, `info`, or `debug`.

## Documentation

- [Configuration reference](docs/configuration.md)
- [Google login setup](docs/google-login.md)
- [Keybindings](docs/keybindings.md)
- [Thumbnail support and limitations](docs/thumbnails.md)
- [Shared queue format and Ytrawl integration](docs/shared-queue.md)
- [Home feed cache and API errors](docs/home-feed.md)
- [Architecture](docs/architecture.md)

> **Troubleshooting tip:** YouTube changes often. If search or playback stops working, update yt-dlp first.

## License

[MIT](LICENSE)
