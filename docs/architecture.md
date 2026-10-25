# Architecture

ytui is a terminal YouTube client built with Bubble Tea. It uses external `yt-dlp` and `mpv` binaries for metadata, streaming, and playback.

## Search and metadata

ytui shells out to:

```bash
yt-dlp --dump-json --flat-playlist
```

It uses `ytsearchN:query` for searches. This approach does not require a YouTube API key for basic search.

## Playback

ytui launches `mpv` with:

```text
--input-ipc-server
```

It communicates with mpv over a unique Unix socket under:

```text
/tmp/ytui-mpv-*.sock
```

This enables pause, seek, volume, and quit controls without leaving the TUI. mpv’s own yt-dlp hook resolves the actual stream; ytui passes it the watch-page URL.

## Local data

Subscriptions, Watch Later, and history are stored as plain JSON at:

```text
~/.config/ytui/data.json
```

Nothing is sent anywhere by ytui itself.

## Now Playing bar

While media is playing, the footer shows a two-line status bar:

- Title, playback mode, and volume
- Progress bar with elapsed and total time

It updates once per second by querying mpv for position, duration, volume, and pause state.

## Roadmap

- [x] Search, playback, pause, and seek through mpv IPC
- [x] Local subscriptions, Watch Later, and history
- [x] Home feed aggregating recent uploads across subscriptions
- [x] Download queue with yt-dlp background downloads and live progress
- [x] Resume playback position
- [x] JSON configuration for download defaults
- [x] OAuth login for real YouTube subscriptions, playlists, and liked videos
- [ ] Desktop notifications through `notify-send`
- [ ] SQLite storage once query needs outgrow flat JSON

## Known limitations

- Flat playlist search results do not always include reliable channel metadata.
- Kitty thumbnail previews can occasionally flicker or lag during rapid selection changes.
- Only one mpv instance runs at a time by design.