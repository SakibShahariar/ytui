# Shared queue with Ytrawl

ytui and the Ytrawl GNOME Shell extension can exchange pending download requests through an optional JSON inbox. No daemon or D-Bus service is required.

## How to use it

1. In Ytrawl, paste a URL and extract its media information.
2. Press **Send to ytui**. Ytrawl appends one request to the shared inbox; it does not start a download in ytui.
3. Start ytui, or open the colon command palette and choose **Import shared queue from Ytrawl**.
4. Accepted requests move into ytui's persistent Downloads queue. ytui removes only the accepted request IDs from the inbox.

The inbox is also useful when ytui is not installed or not running yet: Ytrawl writes an ordinary file and reports write errors without affecting its local download queue. ytui continues to work normally if the inbox is missing, empty, malformed, or temporarily locked.

## Location and concurrency

The default path is $XDG_STATE_HOME/ytui/shared-queue.json, or ~/.local/state/ytui/shared-queue.json when XDG_STATE_HOME is unset. Both applications honor the same XDG state directory. ytui also accepts an absolute YTUI_SHARED_QUEUE_PATH override for tests or deliberate custom setups.

Writers use the sibling shared-queue.json.lock lock file, then atomically replace the JSON document. Locks older than 30 seconds are treated as stale. The state directory is restricted to the current user (0700), and the queue and lock files use 0600 permissions. The queue is an inbox, not a continuously mirrored view of both apps' in-memory queues.

Each item has a stable ID. ytui enqueues it under an ID derived from that value, so if the app crashes after accepting an item but before acknowledging it, a later import does not create a duplicate download. Acknowledgement removes only the IDs that were accepted; concurrently appended requests remain in the file.

## Version 1 format

```json
{
  "version": 1,
  "items": [
    {
      "id": "9b47f89c-26e5-4c66-b9c1-94f35d2a9c20",
      "url": "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
      "title": "Example video",
      "mode": "video",
      "quality": "1080p",
      "audio_format": "",
      "created_at": "2026-10-28T01:23:45.000Z"
    }
  ]
}
```

Top-level version must be 1. items is an ordered array. Required fields are id and an absolute public HTTP/HTTPS URL. mode is video or audio; quality currently maps best, 1080p, 720p, and 480p to ytui's supported download formats. audio_format is used for audio-only requests. Titles and timestamps are metadata, not trusted commands. Unsupported versions, unsafe URLs, embedded URL credentials, and oversized files are rejected without starting downloads.

The JSON file is the interoperability contract. Ytrawl may keep its own queue and history independently; sharing a request never makes either application depend on the other being installed or running.
