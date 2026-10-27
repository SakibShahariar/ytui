# Thumbnails

ytui can show video thumbnails in the Search tab when running inside **Kitty** or a terminal with compatible Kitty graphics-protocol support.

## Requirements

- Kitty terminal, or a terminal with partial Kitty graphics compatibility such as WezTerm.
- The `kitty` executable must be available where applicable.

Most terminals, including GNOME Terminal, do not support the Kitty graphics protocol. In those terminals, ytui shows a “no preview” placeholder instead.

## Usage

1. Select a video in the Search tab.
2. ytui displays its thumbnail in the preview panel on the right.
3. Press `Ctrl+T` to toggle the preview panel if you prefer more list width.

## Cache

Thumbnails are cached at:

```text
~/.cache/ytui/thumbs/
```

Selecting the same video again does not re-download its thumbnail.

By default, the cache keeps at most **250 images** and expires images older than **30 days**. When limits are exceeded, expired images are removed first, then the least-recently-used images. The currently displayed preview is protected from cleanup. Stale temporary files left by interrupted writes are removed after 24 hours.

Override the limits in `$XDG_CONFIG_HOME/ytui/config.json` (or `~/.config/ytui/config.json`):

```json
{
  "thumbnail_cache_max_entries": 250,
  "thumbnail_cache_max_age_days": 30
}
```

Run `ytui doctor` to see the cache directory, current image count, and total cache size. Cache cleanup runs when thumbnails are reused or displayed; the doctor command reports usage without deleting files.

## Known limitations

Bubble Tea and Kitty’s image layer are separate rendering systems writing to the same terminal. As a result, thumbnails may occasionally flicker or lag one frame behind selection changes, especially while scrolling quickly.

This is an inherent limitation of combining terminal image protocols with TUI frameworks, rather than a bug that can be completely eliminated.