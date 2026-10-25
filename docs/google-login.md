# Google login

ytui works without Google login using local, manually tracked subscriptions, Watch Later, and history.

Signing in switches the **Subs**, **Playlists**, and **Liked** tabs to your actual YouTube account data. ytui requests read-only access and never modifies your account.

## One-time setup

1. Open the [Google Cloud Console](https://console.cloud.google.com/) and create or select a project.
2. Go to **APIs & Services → Library**.
3. Search for **YouTube Data API v3** and enable it.
4. Go to **APIs & Services → Credentials** (or **Google Auth Platform → Clients** in the newer Console UI).
5. Create an **OAuth client ID**.
6. If prompted, configure the consent screen:
   - Choose **External**.
   - Enter an application name, such as `ytui`.
   - Add your email address.
   - Save the configuration.
7. For application type, choose **Desktop app**.
8. Download the client-secret JSON file. Its name normally looks like:

   ```text
   client_secret_XXXX.json
   ```

   It is usually saved to `~/Downloads`.

You do not need to move or rename the downloaded file manually.

## Logging in

1. Start ytui.
2. Press `Ctrl+L`.
3. On first login, follow the in-app setup instructions.
4. Press `Enter`, paste the path to the downloaded client-secret JSON file, then press `Enter` again.
5. ytui copies the file to:

   ```text
   ~/.config/ytui/client_secret.json
   ```

6. ytui opens your browser for Google’s consent screen.
7. Approve access, then return to the terminal.

ytui stores its login token at:

```text
~/.config/ytui/token.json
```

It refreshes the token automatically. You should not need to sign in again unless you explicitly log out or revoke access in your Google Account settings.

## Logging out

Press `Ctrl+L` again while already signed in to log out.

You can also revoke ytui’s access from your Google Account’s connected-apps settings.

## Requested access

ytui requests the read-only YouTube scope:

```text
https://www.googleapis.com/auth/youtube.readonly
```

This allows ytui to read your subscriptions, playlists, and liked videos. It cannot modify your account.

## Playlists tab

- The Playlists tab shows your playlists.
- Press `Enter` on a playlist to view its videos.
- Press `Esc` to return to the playlist list.