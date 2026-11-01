<div align="center">

# ytui

**A keyboard-first YouTube client for your terminal.**

Search and play with mpv, manage your library, and queue downloads without opening a browser.

[![Go tests](https://github.com/SakibShahariar/ytui/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/SakibShahariar/ytui/actions/workflows/tests.yml)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

## Features

- **Search and playback** — YouTube search, playlists, direct URLs, and mpv playback.
- **Personal library** — subscriptions, Watch Later, history, and offline downloads.
- **Download queue** — persistent queue, concurrent transfers, retries, and retained partial files.
- **Playback tools** — autoplay, subtitle selection, transcripts, and optional SponsorBlock support.
- **Google integration** — read-only YouTube access with isolated profiles for multiple accounts.
- **Terminal UI** — keyboard-driven navigation, command palette, and optional Kitty-compatible thumbnails.
- **Ytrawl integration** — import media URLs through a shared, permission-restricted queue.
- **Diagnostics** — `ytui doctor` checks dependencies and local state without printing credentials.

## Requirements

ytui requires Go 1.24+ to build from source. Playback and downloads use external tools:

- [Go](https://go.dev/)
- [mpv](https://mpv.io/)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

On Fedora:

```bash
sudo dnf install golang mpv yt-dlp
```

## Install

Install the latest version available from the Go module:

```bash
go install github.com/SakibShahariar/ytui@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`, then run:

```bash
ytui
```

To build from source:

```bash
git clone https://github.com/SakibShahariar/ytui.git
cd ytui
go build -o ytui .
./ytui
```

## Releases

Prebuilt archives are published for Linux and macOS on `amd64` and `arm64`. Each archive includes the executable, README, license, and documentation.

[Browse GitHub releases](https://github.com/SakibShahariar/ytui/releases)

Verify an archive with the `SHA256SUMS` file attached to its release. Release binaries still require mpv and yt-dlp; neither dependency is bundled.

## Essential keys

| Key | Action |
|---|---|
| `Enter` | Search or play the selected item |
| `↑` / `↓` | Navigate lists |
| `Tab` / `Shift+Tab` | Switch tabs |
| `/` | Jump to Search |
| `:` | Open the command palette |
| `?` | Show keyboard help |
| `Ctrl+D` | Open download options |
| `Ctrl+P` | Pause or resume playback |
| `Ctrl+X` | Stop playback |
| `Ctrl+L` | Google login or logout |
| `Ctrl+B` | SponsorBlock settings |
| `Ctrl+J` | Skip the active SponsorBlock segment |
| `Ctrl+V` | Choose subtitles |
| `Ctrl+E` | Open transcript, when available |
| `Ctrl+C` | Quit |

Paste a YouTube playlist URL into Search to browse its videos. Press `Ctrl+W` to add a video to Watch Later. On that tab, `s` cycles sorting, `Space` selects entries, `m` removes selected entries after confirmation, and `x` then `y` clears the queue.

[Complete keybinding guide](docs/keybindings.md)

## Google accounts

ytui works without signing in. Google login adds access to subscriptions, playlists, and liked videos.

[Google login setup](docs/google-login.md)

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

Settings are normally stored in `~/.config/ytui/config.json`. Cache and application data use XDG-compatible directories. Change the download directory with `download_dir` or pass `--dir PATH` for a one-run override.

```bash
ytui --version
ytui doctor
ytui --show-paths
ytui config export ~/ytui-settings.json
ytui config import ~/ytui-settings.json
```

Config exports contain preferences only — not OAuth tokens, credentials, subscriptions, history, or Watch Later entries. Logging is off by default; enable it with `--log-level error`, `info`, or `debug`. Playlist metadata is streamed and capped at 5,000 entries by default; set `YTUI_PLAYLIST_MAX_ENTRIES` to a value from 1 to 50,000 to change the limit.

## Documentation

- [Configuration reference](docs/configuration.md)
- [Google login setup](docs/google-login.md)
- [Keybindings](docs/keybindings.md)
- [Thumbnail support and limitations](docs/thumbnails.md)
- [Shared queue format and Ytrawl integration](docs/shared-queue.md)
- [Home feed cache and API errors](docs/home-feed.md)
- [Architecture](docs/architecture.md)

## Troubleshooting

YouTube changes often. If search or playback stops working, update yt-dlp first.

## License

[MIT](LICENSE)
