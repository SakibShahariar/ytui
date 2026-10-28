# Keybindings

ytui uses a modeless interaction model: on the **Search** tab, typing any letter or digit automatically starts or continues editing the search query.

Pressing an arrow key or `Enter` to play returns control to list browsing. Because ordinary letters can appear in a query, most global shortcuts use `Ctrl` combinations and work on every tab.

## Global keys

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle Search → Subs → Watch Later → History |
| `/` | Jump to the Search tab and start typing |
| `:` | Open the searchable command palette from a main tab |
| `Ctrl+S` | Subscribe to the selected video’s channel |
| `Ctrl+W` | Add the selected video to Watch Later |
| `Ctrl+P` | Pause or resume current playback |
| `Ctrl+N` | Toggle autoplay next; saves the preference |
| `Ctrl+A` | Toggle audio-only mode for the next play |
| `Ctrl+T` | Toggle the thumbnail preview panel |
| `Ctrl+D` | Open download options, or cancel a download from the Downloads tab |
| `Ctrl+R` | Retry a failed/canceled download or resume a retained partial download |
| `Ctrl+Space` | Pause/resume the selected queued or active download |
| `Ctrl+↑` / `Ctrl+↓` | Move the selected queued download up/down |
| `Ctrl+O` | Switch Downloads between the live queue and saved offline files |
| `Ctrl+L` | Log in with Google, or log out if already signed in |
| `Ctrl+X` | Stop current playback |
| `Ctrl+C` | Quit ytui and stop playback |

## Command palette

Press `:` from a main tab to open the command palette. Type to filter actions, use `↑` / `↓` to select, press `Enter` to execute, or `Esc` to close and return to the previous tab. The palette includes navigation, Home refresh, playback/display toggles, library filters, and keyboard help.

## Search tab

Paste a YouTube playlist URL into the Search field and press `Enter` to load its videos in playlist order. The list title shows the playlist name and each entry shows its position. You can play an entry, add it to Watch Later with `Ctrl+W`, or download it with `Ctrl+D`. Private, unavailable, or empty playlists show an error instead of silently appearing as an empty search.


| Key | Action |
|---|---|
| Any letter or digit | Start or continue typing a search query |
| `Enter` | Run the search while typing; play the selected video otherwise |
| `Esc` | Cancel typing and return to browsing; does not quit ytui |
| `↑` `↓` `Page Up` `Page Down` `Home` `End` | Move the list selection; also exits typing mode |

## Playback autoplay

Autoplay is disabled by default. With it enabled, ytui plays the next Watch Later entry first, then the next playable item after the current selection. `Ctrl+X` is an explicit stop and does not trigger autoplay. Local downloaded files are not followed by online videos. The preference is stored as `autoplay_next` in `config.json`.

## Watch Later queue

On the **Watch Later** tab, press `s` to cycle through oldest-added, title A–Z, shortest-duration, and channel A–Z sorting. The chosen order is saved with your local data. Press `Space` to select/deselect entries, then `m` to remove all selected entries in one operation; confirm with `y` or cancel with any other key. Press `x`, then `y` to confirm clearing the entire queue; any other key cancels. Items without saved duration metadata sort after entries with known durations. Adding duplicate videos remains a no-op.

## Lists

On the Subs, Watch Later, and History tabs, `j` and `k` also move through the list in addition to the arrow keys.

## Live download queue

On the live Downloads tab, `Ctrl+Space` pauses or resumes the selected queued or active transfer on supported Unix systems. `Ctrl+↑` and `Ctrl+↓` reorder pending entries; the queue runs one download at a time in that order. `Ctrl+G` cycles status filters.

## Subtitles and transcripts

Select a video and press `Ctrl+V` to inspect available manual subtitles and automatic captions. Choose a language with `↑` / `↓` and `Enter`; the selected language is used for subsequent playback and for downloads when the subtitle option is enabled in `Ctrl+D`. Press `Ctrl+E` to fetch and read the transcript in the TUI. The transcript viewer scrolls with `↑` / `↓`, pages with `PgUp` / `PgDn`, jumps with `Home` / `End`, and closes with `Esc` or `q`. Caption availability depends on yt-dlp and the video; errors are reported without interrupting playback.

## Offline downloads

| Key | Action |
|---|---|
| `Enter` | Play the selected downloaded file with mpv |
| `Delete` | Request deletion of the selected file |
| `y` | Confirm deletion |

`?` opens or closes the in-app keyboard help when you are not typing a search query. Use arrow keys or Page Up/Page Down to scroll, Home/End to jump, and Esc to close.

## History and download filters

On **History** and **Downloads**, press `Ctrl+F` to search titles and channels (offline files are searchable by filename). Press `Ctrl+Y` to cycle the date filter between all dates, the last 7 days, 30 days, and 90 days. History uses the watched date; live downloads use their enqueue time; offline files use modification time. Press `Enter` to apply the text filter, `Esc` to cancel editing, or `Ctrl+U` to clear all library filters. On the live Downloads queue, `Ctrl+G` cycles status filters: all, active, finished, and failed/resumable. Text and status filters combine; changing a filter preserves the current selection where possible.
