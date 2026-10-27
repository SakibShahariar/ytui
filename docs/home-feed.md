# Home feed cache and API errors

The Home tab caches the last non-empty feed in the user cache directory as `ytui/home-feed.json`. The file is private to the current user and written atomically.

- **Fresh cache:** for 10 minutes, revisiting Home reuses the saved feed rather than immediately fetching the full feed again.
- **Stale cache:** the saved feed appears first while ytui refreshes it in the background. The status line indicates that the cache is stale and refreshing.
- **Successful refresh:** cached entries and the timestamp are updated.
- **Failed refresh:** the previous feed remains visible and the error is shown. Quota and authorization failures receive an explicit label.

The cache is keyed to the current set of subscription channel IDs, so switching accounts or subscription sets won't reuse a previous feed. If the cache is missing, corrupt, or belongs to another subscription set, ytui fetches a fresh feed. Home still requires Google sign-in and uses YouTube Data API quota.
