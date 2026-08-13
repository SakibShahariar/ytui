# ytui

A terminal YouTube client: search, play (via mpv), subscribe, watch-later
queue, and history — no browser needed.

## Requirements (Fedora)

```bash
sudo dnf install golang mpv yt-dlp
```

> yt-dlp changes fast with YouTube. If searches/playback break, first try:
> `sudo dnf upgrade yt-dlp`, or `pip install --user -U yt-dlp` for the
> bleeding-edge version.

## Build & run

```bash
cd ytui
go mod tidy    # downloads bubbletea/bubbles/lipgloss — needs network
go build -o ytui .
./ytui
```

Or just `go run .` while developing.

## Keybindings

No mode to think about, no toggle key: on the **Search** tab, typing any
letter or digit automatically starts (or continues) editing your query.
Pressing an arrow key or `enter`-to-play automatically hands control back
to browsing the list. Because plain letters can appear inside a query at
any moment, the old single-letter shortcuts live on `ctrl+` combos instead
— they work identically on every tab and never collide with what you're
typing.

| Key                | Action                                          |
|--------------------|--------------------------------------------------|
| *(any letter/digit)* | starts typing a search query (Search tab)      |
| `↑` `↓` `pgup` `pgdown` `home` `end` | move the list selection; also exits typing mode if you were mid-query |
| `enter`            | while typing: run the search · otherwise: play the selected video |
| `esc`               | cancel typing, back to browsing (does **not** quit) |
| `tab` / `shift+tab`| cycle Search → Subs → Watch Later → History (works from either state) |
| `/`                | jump to the Search tab and start typing, from any other tab |
| `ctrl+s`           | subscribe to the selected video's channel        |
| `ctrl+w`           | add the selected video to Watch Later            |
| `ctrl+p`           | pause/resume current playback                    |
| `[` `]`            | seek -10s / +10s                                 |
| `-` `=`            | volume down / up                                 |
| `ctrl+a`           | toggle audio-only mode for the next play         |
| `ctrl+t`           | toggle the thumbnail preview panel               |
| `ctrl+d`           | open download options (quality/subs/thumbnail) for the selected video — or cancel it, from the Downloads tab |
| `ctrl+l`           | log in with Google (or log out, if already in)   |
| `ctrl+x`           | stop current playback                            |
| `ctrl+c`           | quit, always                                     |

On the Subs / Watch Later / History tabs (no text box there), `j`/`k` also
work for list movement, in addition to the arrow keys.

## Google Login (real subscriptions, playlists, liked videos)

Everything works without this — Subs/Watch Later/History default to a
local, manually-tracked list. Logging in swaps the **Subs**, **Playlists**,
and **Liked** tabs over to your actual YouTube account data.

### One-time setup (~3 minutes, free)

1. Go to the [Google Cloud Console](https://console.cloud.google.com/) and
   create a project (or pick an existing one).
2. **APIs & Services → Library** → search "YouTube Data API v3" → **Enable**.
3. **APIs & Services → Credentials** (or **Google Auth Platform → Clients**
   in newer Console UIs) → **Create Credentials → OAuth client ID**.
   - If prompted, set up the consent screen first — choose **External**,
     fill in an app name (anything, e.g. "ytui"), your email, and save.
     You don't need to submit it for verification; it works fine for your
     own account while the app is in "Testing" mode.
   - Application type: **Desktop app**.
4. Click the download icon (⬇) next to your new client to get its JSON
   file — it's named something like `client_secret_XXXX.json`, and lands
   wherever your browser saves downloads (usually `~/Downloads`).

You don't need to move or rename it yourself — see "Logging in" below.

### Logging in

Press **`ctrl+l`** in ytui.

- If it's your first time, ytui shows the setup steps above right in the
  app. Press `enter`, paste the path to the file you just downloaded
  (e.g. `~/Downloads/client_secret_1234.json`), and press `enter` again —
  ytui copies it into place at `~/.config/ytui/client_secret.json` and
  immediately opens your browser to Google's consent screen.
- Approve access, and the browser tab tells you to close it and go back to
  the terminal.

ytui saves the login to `~/.config/ytui/token.json` and refreshes it
automatically — you shouldn't need to log in again unless you explicitly
log out (`ctrl+l` again while logged in) or revoke access from your Google
Account settings.

The requested scope is read-only
(`https://www.googleapis.com/auth/youtube.readonly`) — ytui can see your
subscriptions/playlists/liked videos, never modify anything on your
account.

### Playlists tab

Shows your playlists at the top level; press `enter` on one to see its
videos, `esc` to go back to the playlist list.

## Now Playing bar

While something's playing, the footer shows a live 2-line bar instead of a
plain status message: title, mode (video/audio), and volume on the first
line; a real progress bar with elapsed/total time on the second. Updates
once per second by querying mpv directly (position, duration, volume,
pause state).

## Config file

`~/.config/ytui/config.json` — optional, created and edited by hand (no
in-app editor for it yet). All fields are optional; anything unset uses
the built-in default.

```json
{
  "download_dir": "/home/you/Videos/ytui",
  "default_quality": "1080p",
  "default_cookies_browser": "firefox",
  "default_audio_only": false
}
```

- `download_dir` — where `ctrl+d` downloads are saved. Default: `~/Downloads/ytui`.
- `default_quality` — pre-selects a row in the download options picker.
  Must match one of the picker's labels exactly: `"Best quality (video+audio)"`,
  `"1080p"`, `"720p"`, `"480p"`, `"Audio only (mp3)"`, `"Audio only (opus)"`.
  Default: best quality.
- `default_cookies_browser` — overrides the picker's browser
  auto-detection (`"firefox"`, `"chrome"`, `"chromium"`, `"brave"`, or
  `"edge"`). Default: auto-detected from what's installed.
- `default_audio_only` — starting state of audio-only playback mode
  (still toggleable in-session with `ctrl+a`). Default: `false`.

## Thumbnail previews (Kitty terminal only)

If you're running ytui inside **Kitty**, selecting a video in the Search
tab shows its thumbnail in a preview panel on the right, drawn via Kitty's
graphics protocol (`kitty +kitten icat`). This is genuinely Kitty-specific
— GNOME Terminal, and most other terminals, have no image protocol at all,
so the panel shows a "no preview" placeholder there instead. WezTerm has
partial Kitty-graphics compatibility and may also work.

Toggle the panel with `t` if you'd rather have the extra list width instead.

Thumbnails are cached at `~/.cache/ytui/thumbs/` so re-selecting a video
doesn't re-download it.

**Known rough edge**: because Bubbletea (the TUI framework) and Kitty's
image layer are two separate rendering systems writing to the same
terminal, you may occasionally see a thumbnail flicker or lag a frame
behind selection changes, especially when moving through the list quickly.
This is inherent to how terminal image protocols currently work alongside
TUI frameworks — not a bug we can fully eliminate, only minimize.

## How it works

- **Search & metadata**: shells out to `yt-dlp --dump-json --flat-playlist`
  against `ytsearchN:query` — fast, no API key needed.
- **Playback**: launches `mpv` with `--input-ipc-server` and talks to it
  over a Unix socket at `/tmp/ytui-mpv.sock`, so pause/seek/quit happen
  without leaving the TUI. mpv's own yt-dlp hook resolves the actual
  stream, so we just hand it the watch-page URL.
- **Local data**: subscriptions / history / watch-later are stored as
  plain JSON at `~/.config/ytui/data.json`. Nothing is sent anywhere.

## Roadmap (see project chat for full plan)

- [x] Search + play + pause/seek via mpv IPC
- [x] Local subscriptions / watch later / history (JSON)
- [x] "New uploads" feed for subscribed channels — Home tab aggregates
      recent uploads across all subscriptions (concurrent fetch, sorted by
      recency), and Subs → enter on a channel shows just that channel's uploads
- [x] Download queue (yt-dlp background downloads + live progress bar,
      `ctrl+d` to queue/cancel)
- [x] Resume playback position (saved every 20s + on stop/quit, auto-seeks
      back on replay)
- [ ] Config file (`~/.config/ytui/config.toml`) for default quality,
      download dir, keybinds
- [ ] Desktop notifications via `notify-send` for new uploads (GNOME-native)
- [ ] Migrate storage to SQLite once query needs grow beyond flat JSON
- [x] OAuth login to pull real YouTube subscriptions, playlists, and liked
      videos, alongside the local-only mode

## Known limitations of this first cut

- `--flat-playlist` search results don't include `channel` reliably for
  every video (YouTube's own metadata inconsistency) — some entries may
  show a blank channel field until we do a secondary metadata fetch.
- No thumbnail rendering yet (would use sixel/kitty graphics protocol,
  terminal-dependent — GNOME Terminal doesn't support either natively;
  worth trying in Kitty or Wezterm if you want that later).
- Single mpv instance at a time (by design, for now).
