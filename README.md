# ytui

A terminal YouTube client: search, play with mpv, manage subscriptions, Watch Later, history, and downloads—without opening a browser.

## Contents

- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Essential keys](#essential-keys)
- [Google login](#google-login)
- [Configuration](#configuration)
- [Downloads](#downloads)
- [Thumbnails](#thumbnails)
- [Documentation](#documentation)
- [License](#license)

## Requirements

- Go
- [mpv](https://mpv.io/)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

On Fedora:

```bash
sudo dnf install golang mpv yt-dlp
```

> YouTube changes frequently. If search or playback stops working, update yt-dlp first.

## Quick start

Install the latest release directly with Go (requires Go 1.22 or newer):

```bash
go install github.com/SakibShahariar/ytui@latest
```

Make sure your Go bin directory is on `PATH`, then run `ytui`. Check the installed build with `ytui --version`, locate its settings with `ytui --show-paths`, or run `ytui doctor` to collect dependency, configuration, authentication-state, and storage diagnostics without printing credentials. You still need `mpv` and `yt-dlp` installed separately.

To build from source instead:

```bash
git clone https://github.com/SakibShahariar/ytui.git
cd ytui
go build -o ytui .
./ytui
```

For development, run `go run .` from the cloned repository.

## Essential keys

| Key | Action |
|---|---|
| `Enter` | Search while typing; play the selected video otherwise |
| `↑` / `↓` | Move through lists |
| `Tab` / `Shift+Tab` | Switch tabs |
| `/` | Jump to Search |
| `Ctrl+D` | Download options |
| `Ctrl+P` | Pause or resume |
| `Ctrl+X` | Stop playback |
| `Ctrl+L` | Google login / logout |
| `Ctrl+C` | Quit ytui and stop the current mpv playback session |

See the [full keybinding reference](docs/keybindings.md).

## Google login

ytui works without Google login using local subscriptions, Watch Later, and history.

To use your real YouTube subscriptions, playlists, and liked videos, press `Ctrl+L` and follow the in-app setup. ytui requests read-only YouTube access and stores its token locally.

Full setup instructions: [Google login](docs/google-login.md).

## Configuration

Configuration lives at:

```text
$XDG_CONFIG_HOME/ytui/config.json
```

By default, that is:

```text
~/.config/ytui/config.json
```

Common settings include the download directory, default quality, preferred cookie browser, and audio-only playback.

```json
{
  "download_dir": "~/Videos/ytui",
  "default_quality": "1080p",
  "default_cookies_browser": "firefox",
  "default_audio_only": false
}
```

Export or import preferences:

```bash
ytui --export-config ~/ytui-settings.json
ytui --import-config ~/ytui-settings.json
```

Configuration export contains preferences only; OAuth tokens, credentials, subscriptions, history, and the Watch Later queue are not included.

Full reference: [Configuration](docs/configuration.md).

## Downloads

Press `Ctrl+D` to queue a download. Use the Downloads tab and `Ctrl+O` to switch between the active queue and saved offline files.

Downloads are saved to:

```text
~/Downloads/ytui
```

You can change this with `download_dir` in the configuration file.

## Thumbnails

In Kitty and compatible terminals, ytui can show video thumbnails using Kitty’s graphics protocol. Toggle the preview panel with `Ctrl+T`.

Thumbnails are cached under:

```text
~/.cache/ytui/thumbs/
```

Details and limitations: [Thumbnails](docs/thumbnails.md).

## Documentation

- [Full keybindings](docs/keybindings.md)
- [Google login setup](docs/google-login.md)
- [Configuration reference](docs/configuration.md)
- [Thumbnails](docs/thumbnails.md)
- [Architecture](docs/architecture.md)
- Diagnostics: run `ytui doctor` for dependency versions, config/data paths, authentication state, and download-directory status without printing credentials.

## License

MIT. See [LICENSE](LICENSE).
