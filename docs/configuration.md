# Configuration

ytui stores its optional configuration file at:

```text
$XDG_CONFIG_HOME/ytui/config.json
```

When `XDG_CONFIG_HOME` is unset, this becomes:

```text
~/.config/ytui/config.json
```

All fields are optional. Any unset field uses ytui’s built-in default.

## Example

```json
{
  "download_dir": "/home/you/Videos/ytui",
  "max_concurrent_downloads": 3,
  "default_quality": "1080p",
  "default_cookies_browser": "firefox",
  "default_audio_only": false,
  "desktop_notifications": false,
  "autoplay_next": false,
  "thumbnail_cache_max_entries": 250,
  "thumbnail_cache_max_age_days": 30,
  "download_presets": [
    { "name": "Lecture", "quality": "720p", "write_subs": true },
    { "name": "Podcast", "quality": "Audio only (mp3)", "rate_limit": "500K" }
  ]
}
```

## Settings

| Field | Description | Default |
|---|---|---|
| `download_dir` | Directory where `Ctrl+D` downloads are saved. `--dir DIR` overrides it for one run. | `~/Downloads/ytui` |
| `max_concurrent_downloads` | Maximum simultaneous yt-dlp processes. Accepted values are `1`–`10`; `0` keeps the single-download default. | `1` |
| `download_rate_limit` | Global yt-dlp download rate limit, e.g. `500K` or `2M`. A positive number with an optional `K`, `M`, or `G` suffix. The `Ctrl+D` picker can override it per download. | unlimited |
| `download_retry_attempts` | Automatic retries after a transient yt-dlp failure (network errors, timeouts, rate limits). `0` disables retries; maximum `10`. Permanent failures are never retried. | `0` |
| `download_retry_backoff_seconds` | Base delay before the first retry, doubled on each subsequent attempt. `0` uses five seconds. | `5` |
| `keybindings` | Object mapping global shortcut actions (for example `quit`, `download`, `search`) to key strings. See [Keybindings](keybindings.md#configurable-shortcuts). | built-in defaults |
| `download_presets` | List of custom download presets (`name`, `quality`, `write_subs`, `embed_thumbnail`, `rate_limit`) added to the `Ctrl+D` preset row. | built-in presets only |
| `mouse_enabled` | Opt into mouse wheel scrolling and click-to-select in the TUI. Keyboard controls always work. | `false` |
| `theme` | Built-in color profile: `default`, `minimal`, `high-contrast`, or `light`. Also switchable from the command palette. | `default` |
| `colors` | Object overriding individual palette roles with `#RGB` or `#RRGGBB` values, e.g. `"mauve": "#ff00ff"`. Roles: base, mantle, text, subtext1, subtext0, overlay1, overlay0, surface2, surface1, mauve, pink, red, peach, yellow, green, blue, lavender. | theme defaults |
| `default_quality` | Preselected row in the download-options picker. | Best quality |
| `default_cookies_browser` | Overrides browser auto-detection for cookies. | Auto-detected |
| `default_audio_only` | Initial audio-only playback state. | `false` |
| `disable_resume_prompt` | Start videos from the beginning instead of prompting to resume saved progress. Progress is still saved. | `false` |
| `desktop_notifications` | Send best-effort Linux desktop notifications when downloads finish or fail. Requires `notify-send`; notifications are disabled if false. | `false` |
| `autoplay_next` | Automatically play the next Watch Later item, or the next item in the current list, when playback ends. Toggle with `Ctrl+N`. | `false` |
| `download_sort` | Live Downloads queue order: `added`, `title`, `channel`, `progress`, or `status`. Press `s` on Downloads to cycle. | `added` |
| `download_status_filter` | Live Downloads status filter: `0` all, `1` active, `2` finished, `3` failed/resumable. Press `Ctrl+G` to cycle. | `0` |
| `thumbnail_cache_max_entries` | Maximum cached thumbnail images; oldest unused entries are removed first. | `250` |
| `thumbnail_cache_max_age_days` | Maximum age of cached thumbnails before cleanup. | `30` |

## SponsorBlock

SponsorBlock is optional and disabled by default. Enable it from the TUI with `Ctrl+B`; changes are saved in `config.json`.

- `sponsorblock_enabled`: whether to look up segments during playback.
- `sponsorblock_categories`: categories to skip automatically. If omitted, `sponsor` and `selfpromo` are the initial defaults. An explicitly empty list disables automatic skipping while keeping segment highlighting and manual skip available.
- `sponsorblock_categories_configured`: internal marker that distinguishes the first-run defaults from an intentionally empty category selection; normally leave it to ytui.

The API lookup uses SponsorBlock's hash-prefix endpoint, sending only four SHA-256 hash characters instead of the raw video ID.

## Quality values

Use a stable identifier for `default_quality`; display labels remain supported for existing configurations:

| Stable identifier | Display label |
|---|---|
| `best` | `Best quality (video+audio)` |
| `1080p` | `1080p` |
| `720p` | `720p` |
| `480p` | `480p` |
| `audio-mp3` | `Audio only (mp3)` |
| `audio-opus` | `Audio only (opus)` |

Legacy values such as `"Best quality (video+audio)"` and `"Audio only (mp3)"` continue to work. Identifiers are normalized when the config is read.

## Browser values

`default_cookies_browser` accepts:

- `"firefox"`
- `"chrome"`
- `"chromium"`
- `"brave"`
- `"edge"`

## Export and import

Export the effective settings:

```bash
ytui --export-config ~/ytui-settings.json
```

Import settings without opening the TUI:

```bash
ytui config import ~/ytui-settings.json
```

Importing replaces the active settings file. By default, ytui asks for confirmation and displays both the source file and destination config path. Enter `y` or `yes` to proceed; any other response cancels without changing settings.

For automation, explicitly opt in to the replacement:

```bash
ytui config import ~/ytui-settings.json --yes
```

The legacy `--import-config FILE` flag remains supported. It imports the settings and then continues into the interactive TUI; use the `config import` subcommand when you want the process to exit after importing.

Before importing a file you did not just export, consider backing up your current configuration. For the default location, for example:

```bash
cp ~/.config/ytui/config.json ~/.config/ytui/config.json.bak
```

If you use `XDG_CONFIG_HOME`, back up the `ytui/config.json` file under that directory instead. Adjust the source path if your configuration file is stored elsewhere.

Imports are validated before the active configuration is replaced. Unsupported `default_quality`, `default_cookies_browser`, or `download_rate_limit` values, and control characters in `download_dir`, produce field-specific errors; the existing settings remain unchanged. The directory does not have to exist at import time, so portable setups can configure a future path. The current config format has no numeric fields or required fields; omitted values use defaults.

Exported files contain preferences only. They do not include:

- OAuth tokens
- Google client credentials
- Subscriptions
- Watch history
- Watch Later queue

An exported preferences file is therefore **not a complete backup** of ytui's local state. Back up any data you need separately.

Configuration files are written with owner-only permissions (`0600`).

## Diagnostic log rotation

When file logging is enabled, ytui writes to `$XDG_CACHE_HOME/ytui/ytui.log` (or `~/.cache/ytui/ytui.log` when `XDG_CACHE_HOME` is unset). The active log is rotated before a write would exceed **5 MiB** by default. The three most recent rotated files are retained as `ytui.log.1`, `ytui.log.2`, and `ytui.log.3`.

Advanced users can override the limits with environment variables:

| Variable | Default | Accepted values |
|---|---:|---|
| `YTUI_LOG_MAX_BYTES` | `5242880` | Integer of at least `1024` bytes |
| `YTUI_LOG_BACKUPS` | `3` | Integer from `1` to `10` |

These settings apply when ytui starts. Rotation is serialized with log writes, and log files remain owner-readable only.

## Related

- [Google login](google-login.md)
- [Keybindings](keybindings.md)


## Large playlist handling

Playlist metadata is decoded entry by entry instead of retaining yt-dlp's complete JSON metadata document. ytui keeps only the compact fields needed by the interface and download queue, and stops with a clear error when a playlist exceeds the configured entry limit.

| Environment variable | Default | Accepted values |
|---|---:|---|
| `YTUI_PLAYLIST_MAX_ENTRIES` | `5000` | Integer from `1` to `50000` |

For example, allow up to 10,000 entries for one launch:

```sh
YTUI_PLAYLIST_MAX_ENTRIES=10000 ytui
```

The limit counts playlist entries, including unavailable entries, so unusually large or partially unavailable playlists cannot bypass the memory bound.


Download sorting and status filtering apply to the live queue. File-size sorting is not available because ytui does not currently retain a reliable final size for each queue entry.

The Downloads view shows the number of active workers and pending queue entries. Queue order remains FIFO unless changed manually with the queue movement controls. Paused active processes continue to occupy a worker slot until resumed or canceled.
