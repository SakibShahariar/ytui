# Keybindings

ytui uses a modeless interaction model: on the **Search** tab, typing any letter or digit automatically starts or continues editing the search query.

Pressing an arrow key or `Enter` to play returns control to list browsing. Because ordinary letters can appear in a query, most global shortcuts use `Ctrl` combinations and work on every tab.

## Global keys

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle Search → Subs → Watch Later → History |
| `/` | Jump to the Search tab and start typing |
| `Ctrl+S` | Subscribe to the selected video’s channel |
| `Ctrl+W` | Add the selected video to Watch Later |
| `Ctrl+P` | Pause or resume current playback |
| `Ctrl+A` | Toggle audio-only mode for the next play |
| `Ctrl+T` | Toggle the thumbnail preview panel |
| `Ctrl+D` | Open download options, or cancel a download from the Downloads tab |
| `Ctrl+O` | Switch Downloads between the live queue and saved offline files |
| `Ctrl+L` | Log in with Google, or log out if already signed in |
| `Ctrl+X` | Stop current playback |
| `Ctrl+C` | Quit ytui and stop playback |

## Search tab

| Key | Action |
|---|---|
| Any letter or digit | Start or continue typing a search query |
| `Enter` | Run the search while typing; play the selected video otherwise |
| `Esc` | Cancel typing and return to browsing; does not quit ytui |
| `↑` `↓` `Page Up` `Page Down` `Home` `End` | Move the list selection; also exits typing mode |

## Lists

On the Subs, Watch Later, and History tabs, `j` and `k` also move through the list in addition to the arrow keys.

## Offline downloads

| Key | Action |
|---|---|
| `Enter` | Play the selected downloaded file with mpv |
| `Delete` | Request deletion of the selected file |
| `y` | Confirm deletion |