# ytui

A terminal YouTube client: search, play with mpv, manage subscriptions, Watch Later, history, and downloads—without opening a browser.

## Contents

- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Releases](#releases)
- [Essential keys](#essential-keys)
- [Google login](#google-login)
- [Configuration](#configuration)
- [Logging](#logging)
- [Downloads](#downloads)
- [Thumbnails](#thumbnails)
- [Documentation](#documentation)
- [License](#license)

## Requirements

- Go 1.24 or newer
- [mpv](https://mpv.io/)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

On Fedora:

```bash
sudo dnf install golang mpv yt-dlp
```

> YouTube changes frequently. If search or playback stops working, update yt-dlp first.

## Quick start

Install the latest tagged release directly with Go (requires Go 1.24 or newer):

```bash
go install github.com/SakibShahariar/ytui@latest
```

Make sure your Go bin directory is on `PATH`, then run `ytui`. See [GitHub Releases](https://github.com/SakibShahariar/ytui/releases) for versioned binaries and checksums. Release binaries require `mpv` and `yt-dlp` to be installed separately. To open a video directly, pass its URL, for example `ytui https://www.youtube.com/watch?v=VIDEO_ID`. Use `ytui --dir /path/to/downloads` to override the download/offline-library directory for this run; the configured `download_dir` remains the default when `--dir` is omitted. Check the installed build with `ytui --version`, locate its settings with `ytui --show-paths`, or run `ytui doctor` to collect dependency, configuration, authentication-state, and storage diagnostics without printing credentials. You still need `mpv` and `yt-dlp` installed separately.

To build from source instead:

```bash
git clone https://github.com/SakibShahariar/ytui.git
cd ytui
go build -o ytui .
./ytui
```

For development, run `go run .` from the cloned repository.

## Releases

Tagged versions publish Linux (`amd64`, `arm64`) and macOS (`amd64`, `arm64`) archives with SHA-256 checksums. Download the archive matching your operating system and CPU architecture plus `SHA256SUMS` from the same release. In the directory containing both downloads, verify the archive with `sha256sum --check SHA256SUMS`, extract it with `tar -xzf ytui-VERSION-OS-ARCH.tar.gz`, and place the `ytui` binary somewhere on your `PATH`. These binaries do not bundle `mpv` or `yt-dlp`; install both separately. Verify an installed version with `ytui --version` or `ytui -v`.

## Playback autoplay

Autoplay is off by default. Press `Ctrl+N` to toggle it; the preference is saved in `config.json` as `autoplay_next`. When enabled, ytui plays the next Watch Later item first, then advances to the next playable item in the current list. Manually stopping playback with `Ctrl+X` does not trigger autoplay. Local downloaded files do not autoplay into online videos.

## SponsorBlock

SponsorBlock is optional and off by default. Press `Ctrl+B` to enable it and choose which categories are automatically skipped. Other available segments are highlighted in the Now Playing bar; press `Ctrl+J` while inside a segment to skip it manually. ytui uses SponsorBlock’s privacy-preserving hash-prefix endpoint, sending only four SHA-256 hash characters rather than the raw video ID. Network errors or missing segment data never stop playback. See the [SponsorBlock API documentation](https://wiki.sponsor.ajay.app/w/API_Docs).

## Shared queue with Ytrawl

Ytrawl can send extracted media URLs into ytui without a running daemon. Its **Send to ytui** action writes a request to the shared inbox; ytui imports it on startup, or you can use the colon command palette action **Import shared queue from Ytrawl** while ytui is already open. Imported requests enter the persistent Downloads queue. If the shared file is absent, ytui behaves normally.

The inbox uses a versioned JSON schema, an exclusive lock file, atomic replacement, and per-user file permissions. See [the format and concurrency contract](docs/shared-queue.md).

## Essential keys

| Key | Action |
|---|---|
| `Enter` | Search while typing; play the selected video otherwise |
| `↑` / `↓` | Move through lists |
| `Tab` / `Shift+Tab` | Switch tabs |
| `/` | Jump to Search |
| `:` | Open the searchable command palette |
| `?` | Open in-app keyboard help |
| `Ctrl+D` | Download options |
| `Ctrl+P` | Pause or resume |
| `Ctrl+X` | Stop playback |
| `Ctrl+B` | SponsorBlock settings and category toggles |
| `Ctrl+J` | Manually skip the active SponsorBlock segment |
| `Ctrl+L` | Google login / logout |
| `Ctrl+C` | Quit ytui and stop the current mpv playback session |

Paste a YouTube playlist URL into Search and press `Enter` to list its videos in order. Select an entry to play it, use `Ctrl+W` to add it to Watch Later, or `Ctrl+D` to download it.

Use `Ctrl+V` to choose a subtitle language for playback/downloads, and `Ctrl+E` to open a video transcript when captions are available.

See the [full keybinding reference](docs/keybindings.md).

## Google login

ytui works without Google login using local subscriptions, Watch Later, and history.

To use your real YouTube subscriptions, playlists, and liked videos, press `Ctrl+L` and follow the in-app setup. ytui requests read-only YouTube access and stores its token locally.

Full setup instructions: [Google login](docs/google-login.md).

Multiple Google accounts are supported through named profiles. The original single-account token remains the `default` profile:

```bash
ytui profile list
ytui profile create work
ytui profile use work
# Start ytui and press Ctrl+L to sign in to that Google account
ytui profile use default
ytui profile remove work  # asks for confirmation
```

The TUI header shows the active profile. OAuth app credentials (`client_secret.json`) are shared, while each profile has its own token file. Profile directories are owner-only and token/selection files use `0600` permissions.

### Download queue recovery

The live download queue is saved to `.ytui-queue.json` inside the configured download directory. Pending order and paused items are restored on the next launch; an interrupted active download is returned to the queue so yt-dlp can continue its partial file. The queue state file is written atomically with owner-only permissions.

## Configuration

Configuration lives at:

```text
$XDG_CONFIG_HOME/ytui/config.json
```

By default, that is:

```text
~/.config/ytui/config.json
```

Common settings include the download directory, default quality, preferred cookie browser, audio-only playback, and optional desktop notifications.

```json
{
  "download_dir": "~/Videos/ytui",
  "default_quality": "1080p",
  "default_cookies_browser": "firefox",
  "default_audio_only": false,
  "desktop_notifications": false
}
```

Export or import preferences:

```bash
ytui --export-config ~/ytui-settings.json
ytui --import-config ~/ytui-settings.json
```

Configuration export contains preferences only; OAuth tokens, credentials, subscriptions, history, and the Watch Later queue are not included.

Full reference: [Configuration](docs/configuration.md).

## Logging

File logging is disabled by default. Enable it when troubleshooting with `--log-level`:

```bash
ytui --log-level error
ytui --log-level info
ytui --log-level debug
```

The log is appended to `$XDG_CACHE_HOME/ytui/ytui.log` (normally `~/.cache/ytui/ytui.log`). Use `--log-level off` to keep logging disabled. The log directory is created with private permissions and the log file is restricted to the current user (`0600`). Error messages are filtered for common credential fields and sensitive URL query parameters; still review logs before sharing them publicly.

## Downloads

Set `desktop_notifications` to `true` in `config.json` to receive best-effort Linux desktop notifications when downloads complete or fail. Notifications use `notify-send` (provided by libnotify); if it is unavailable, downloads continue normally without notifications. Notifications are disabled by default.


Press `Ctrl+D` to queue a download. In the Downloads queue, `Ctrl+D` cancels an active transfer and `Ctrl+R` retries failed/canceled entries or resumes retained partials. `Ctrl+O` switches between the active queue and saved offline files. On startup, ytui removes `.part`, `.ytdl`, and temporary artifacts older than 30 days from the dedicated download directory; newer partials are kept for resuming.

Downloads are saved to:

```text
~/Downloads/ytui
```

You can change this with `download_dir` in the configuration file.

For isolated or portable setups, `ytui --config PATH --data PATH` overrides the config and data files for that run. The equivalent environment variables are `YTUI_CONFIG_PATH` and `YTUI_DATA_PATH`. Command-line overrides take precedence over environment variables; when neither is set, ytui uses the platform's XDG-compatible config directory.

## Thumbnails

In Kitty and compatible terminals, ytui can show video thumbnails using Kitty’s graphics protocol. Toggle the preview panel with `Ctrl+T`.

Thumbnails are cached under:

```text
~/.cache/ytui/thumbs/
```

Details and limitations: [Thumbnails](docs/thumbnails.md).

## CLI commands

The command interface is grouped around a few discoverable subcommands:

- `ytui help` — show commands and legacy flags
- `ytui doctor` — diagnose dependencies and local state
- `ytui paths` — print effective config/data paths
- `ytui config export PATH` — export settings
- `ytui config import PATH` — import settings with confirmation; add `--yes` for automation
- `ytui profile list|create|use|remove` — manage isolated Google login tokens; removal asks for confirmation (`--yes` skips it)

Existing flags such as `--show-paths`, `--export-config`, and `--import-config` remain supported.

## Documentation

- [Full keybindings](docs/keybindings.md)
- [Google login setup](docs/google-login.md)
- [Home feed cache and API errors](docs/home-feed.md)
- [Configuration reference](docs/configuration.md)
- [Thumbnails](docs/thumbnails.md)
- [Architecture](docs/architecture.md)
- Diagnostics: run `ytui doctor` for dependency versions, config/data paths, authentication state, and download-directory status without printing credentials.

## License

MIT. See [LICENSE](LICENSE).

## Watch Later queue controls

On the Watch Later tab, press `s` to cycle through oldest-added, title A–Z, shortest-duration, and channel A–Z sorting. The selected order is persisted locally. Press `Space` to select/deselect items, then `m` to remove all selected entries after confirmation. Press `x`, then `y` to clear the entire queue; any other key cancels either confirmation. Videos without known duration sort after videos with known duration.

