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

## Known limitations

Bubble Tea and Kitty’s image layer are separate rendering systems writing to the same terminal. As a result, thumbnails may occasionally flicker or lag one frame behind selection changes, especially while scrolling quickly.

This is an inherent limitation of combining terminal image protocols with TUI frameworks, rather than a bug that can be completely eliminated.