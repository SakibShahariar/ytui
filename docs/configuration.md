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
  "default_audio_only": false
}
```

## Settings

| Field | Description | Default |
|---|---|---|
| `download_dir` | Directory where `Ctrl+D` downloads are saved. | `~/Downloads/ytui` |
| `default_quality` | Preselected row in the download-options picker. | Best quality |
| `default_cookies_browser` | Overrides browser auto-detection for cookies. | Auto-detected |
| `default_audio_only` | Initial audio-only playback state. | `false` |

## Quality values

`default_quality` must match one of the download picker’s labels exactly:

- `"Best quality (video+audio)"`
- `"1080p"`
- `"720p"`
- `"480p"`
- `"Audio only (mp3)"`
- `"Audio only (opus)"`

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

**Import replaces the active settings.** Before importing a file you did not just export, make a backup of your current configuration. For the default location, for example:

```bash
cp ~/.config/ytui/config.json ~/.config/ytui/config.json.bak
```

If you use `XDG_CONFIG_HOME`, back up the `ytui/config.json` file under that directory instead. Adjust the source path if your configuration file is stored elsewhere.

Import validates the JSON and replaces the active settings before the app starts. Invalid input leaves the current settings unchanged. Additional field-level validation and an explicit import confirmation are tracked in [#39](https://github.com/SakibShahariar/ytui/issues/39) and [#40](https://github.com/SakibShahariar/ytui/issues/40); until those safeguards are implemented, review the file carefully before importing it.

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
