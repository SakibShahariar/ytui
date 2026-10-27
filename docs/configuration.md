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
  "default_quality": "1080p",
  "default_cookies_browser": "firefox",
  "default_audio_only": false,
  "thumbnail_cache_max_entries": 250,
  "thumbnail_cache_max_age_days": 30
}
```

## Settings

| Field | Description | Default |
|---|---|---|
| `download_dir` | Directory where `Ctrl+D` downloads are saved. `--dir DIR` overrides it for one run. | `~/Downloads/ytui` |
| `default_quality` | Preselected row in the download-options picker. | Best quality |
| `default_cookies_browser` | Overrides browser auto-detection for cookies. | Auto-detected |
| `default_audio_only` | Initial audio-only playback state. | `false` |
| `thumbnail_cache_max_entries` | Maximum cached thumbnail images; oldest unused entries are removed first. | `250` |
| `thumbnail_cache_max_age_days` | Maximum age of cached thumbnails before cleanup. | `30` |

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

Import settings before launching ytui:

```bash
ytui --import-config ~/ytui-settings.json
```

Importing replaces the active settings file. By default, ytui asks for confirmation and displays both the source file and destination config path. Enter `y` or `yes` to proceed; any other response cancels without changing settings.

For automation, explicitly opt in to the replacement:

```bash
ytui --import-config ~/ytui-settings.json --yes
```

Before importing a file you did not just export, consider backing up your current configuration. For the default location, for example:

```bash
cp ~/.config/ytui/config.json ~/.config/ytui/config.json.bak
```

If you use `XDG_CONFIG_HOME`, back up the `ytui/config.json` file under that directory instead. Adjust the source path if your configuration file is stored elsewhere.

Imports are validated before the active configuration is replaced. Unsupported `default_quality` or `default_cookies_browser` values, and control characters in `download_dir`, produce field-specific errors; the existing settings remain unchanged. The directory does not have to exist at import time, so portable setups can configure a future path. The current config format has no numeric fields or required fields; omitted values use defaults.

Exported files contain preferences only. They do not include:

- OAuth tokens
- Google client credentials
- Subscriptions
- Watch history
- Watch Later queue

An exported preferences file is therefore **not a complete backup** of ytui's local state. Back up any data you need separately.

Configuration files are written with owner-only permissions (`0600`).

## Related

- [Google login](google-login.md)
- [Keybindings](keybindings.md)
