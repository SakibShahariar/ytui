package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderLogo() string {
	letterColors := []lipgloss.Color{ctpMauve, ctpPink, ctpLavender, ctpBlue}
	var out []string
	for _, line := range logoLines {
		runes := []rune(line)
		var b strings.Builder
		for i, r := range runes {
			idx := i / 6 // 4 letter-blocks of 6 chars each (5 wide + 1 gap)
			if idx > 3 {
				idx = 3
			}
			b.WriteString(lipgloss.NewStyle().Foreground(letterColors[idx]).Bold(true).Render(string(r)))
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

// welcomeContent builds the first-screen view shown before any search has
// run: logo, tagline, a quick glance at the user's library, and a nudge
// toward the / key — instead of an empty box with just a placeholder.
func (m Model) welcomeContent(width, availableHeight int) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	tagline := subtitleStyle.Render("search, watch, and track youtube — never leave the terminal")

	numStyle := lipgloss.NewStyle().Foreground(ctpYellow).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(ctpSubtext0)
	stats := fmt.Sprintf(
		"⭐ %s %s    🕒 %s %s    📜 %s %s",
		numStyle.Render(fmt.Sprint(len(m.store.Subscriptions))), labelStyle.Render("subs"),
		numStyle.Render(fmt.Sprint(len(m.store.WatchLater))), labelStyle.Render("watch later"),
		numStyle.Render(fmt.Sprint(len(m.store.History))), labelStyle.Render("watched"),
	)

	hint := lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).Render("just start typing") +
		lipgloss.NewStyle().Foreground(ctpOverlay1).Render(" to search")

	// Full version: logo(5) + blank + tagline + blank + stats + blank + hint = 12 lines.
	// Compact version (no logo, just a plain title line) = 7 lines.
	// Overflowing past availableHeight is what was pushing the title bar and
	// tabs off the top of short terminals (the terminal itself scrolls to
	// keep the bottom of the overflowing content in view) — this picks
	// whichever version actually fits instead of always using the tallest.
	const fullHeight = 12
	if availableHeight >= fullHeight || availableHeight <= 0 {
		parts := []string{
			center.Render(renderLogo()),
			"",
			center.Render(tagline),
			"",
			center.Render(stats),
			"",
			center.Render(hint),
		}
		return "\n" + strings.Join(parts, "\n")
	}

	parts := []string{
		center.Render(titleStyle.Render("ytui")),
		center.Render(tagline),
		"",
		center.Render(stats),
		"",
		center.Render(hint),
	}
	return "\n" + strings.Join(parts, "\n")
}

// loginHelpContent is the in-app Google Cloud Console setup walkthrough,
// shown when ctrl+l is pressed but no client_secret.json exists yet — no
// need to alt-tab out to a README to figure out what to do.
// hyperlink wraps label in an OSC 8 terminal hyperlink escape sequence —
// Kitty (and most modern terminals) render this as an actual clickable
// link, not just colored text that looks like one.
func hyperlink(url, label string) string {
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

// downloadPickerContent renders the ctrl+d quality/options menu: a radio
// list of quality presets, two checkboxes, and a confirm row — all as one
// flat cursor-navigable list rather than separate widgets.
func (m Model) downloadPickerContent(width int) string {
	heading := lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("Download options")
	title := ""
	if m.downloadPickerVideo != nil {
		title = subtitleStyle.Render(truncate(m.downloadPickerVideo.Title, width))
	}

	cursorStyle := lipgloss.NewStyle().Foreground(ctpMauve).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(ctpText)
	dimStyle := lipgloss.NewStyle().Foreground(ctpOverlay1)

	row := func(idx int, marker, label string) string {
		prefix := "  "
		style := rowStyle
		if idx == m.downloadPickerCursor {
			prefix = cursorStyle.Render("▸ ")
			style = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true)
		}
		return prefix + style.Render(marker+" "+label)
	}

	var lines []string
	lines = append(lines, heading, title, "")

	for i, q := range downloadQualities {
		radio := "○"
		if i == m.downloadPickerQuality {
			radio = "●"
		}
		lines = append(lines, row(i, radio, q.Label))
	}

	qCount := len(downloadQualities)
	lines = append(lines, "")

	subsBox := "[ ]"
	if m.downloadPickerSubs {
		subsBox = "[x]"
	}
	lines = append(lines, row(qCount, subsBox, "Download subtitles"))

	thumbBox := "[ ]"
	if m.downloadPickerThumb {
		thumbBox = "[x]"
	}
	lines = append(lines, row(qCount+1, thumbBox, "Embed thumbnail"))

	cookieLabel := "Cookies from browser: " + downloadCookieBrowserLabels(m.defaultCookieBrowserIndex())[m.downloadPickerCookies]
	lines = append(lines, row(qCount+2, "⟳", cookieLabel))

	lines = append(lines, "")
	startLabel := "▶ Start Download"
	if m.downloadPickerCursor == qCount+3 {
		lines = append(lines, cursorStyle.Render("▸ ")+lipgloss.NewStyle().Foreground(ctpGreen).Bold(true).Render(startLabel))
	} else {
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ctpGreen).Render(startLabel))
	}

	lines = append(lines, "", dimStyle.Render("↑↓: move · enter/space: select/toggle/cycle · esc: cancel"))

	return "\n" + strings.Join(lines, "\n")
}

func (m Model) loginHelpContent(width int) string {
	heading := lipgloss.NewStyle().Bold(true).Foreground(ctpMauve).Render("Set up Google Login")
	sub := subtitleStyle.Render("one-time, ~3 minutes, free — needed for real subs/playlists/liked videos")

	step := func(n int, text string) string {
		num := lipgloss.NewStyle().Bold(true).Foreground(ctpBlue).Render(fmt.Sprintf(" %d ", n))
		return num + "  " + lipgloss.NewStyle().Foreground(ctpText).Render(text)
	}
	sep := dividerStyle.Render(strings.Repeat("─", width))

	linkText := lipgloss.NewStyle().Foreground(ctpBlue).Underline(true).Render("console.cloud.google.com")
	link := hyperlink("https://console.cloud.google.com/", linkText)

	lines := []string{
		heading,
		sub,
		"",
		step(1, "Open "+link+", create (or pick) a project."),
		step(2, "APIs & Services → Library → enable \"YouTube Data API v3\"."),
		step(3, "APIs & Services → Credentials → Create Credentials →"),
		"     " + lipgloss.NewStyle().Foreground(ctpSubtext1).Render("OAuth client ID → Application type: Desktop app"),
		step(4, "Google shows a Client ID and Client Secret — copy both."),
		"",
		lipgloss.NewStyle().Foreground(ctpPeach).Render("⚠ ") +
			lipgloss.NewStyle().Foreground(ctpSubtext1).Render("When you log in, Google will warn \"unverified app.\""),
		"  " + lipgloss.NewStyle().Foreground(ctpSubtext1).Render("That's normal — it's your own app. Click"),
		"  " + lipgloss.NewStyle().Foreground(ctpSubtext1).Render("\"Advanced\" → \"Go to [app] (unsafe)\" to continue."),
		"",
		sep,
	}

	switch m.credentialStep {
	case 1:
		lines = append(lines,
			lipgloss.NewStyle().Foreground(ctpText).Render("Paste your Client ID:"),
			m.pathInput.View(),
			"",
			lipgloss.NewStyle().Foreground(ctpOverlay0).Render("enter: next  ·  esc: cancel"),
		)
	case 2:
		lines = append(lines,
			lipgloss.NewStyle().Foreground(ctpGreen).Render("✓ Client ID saved.")+"  "+
				lipgloss.NewStyle().Foreground(ctpText).Render("Now paste your Client Secret:"),
			m.pathInput.View(),
			"",
			lipgloss.NewStyle().Foreground(ctpOverlay0).Render("enter: import  ·  esc: cancel"),
		)
	default:
		lines = append(lines,
			lipgloss.NewStyle().Foreground(ctpOverlay1).Render("Once you have those, press ")+
				lipgloss.NewStyle().Foreground(ctpGreen).Bold(true).Render("enter")+
				lipgloss.NewStyle().Foreground(ctpOverlay1).Render(" — ytui will ask for")+
				" your Client ID, then your Client Secret.",
			lipgloss.NewStyle().Foreground(ctpOverlay0).Render("(esc to dismiss)"),
		)
	}
	return "\n" + strings.Join(lines, "\n")
}

// notLoggedInContent is the empty state for tabs (Playlists, Liked) that
// have no local-only fallback — instead of a bare "No items.", it explains
// why and points at the fix.
func (m Model) notLoggedInContent(width int) string {
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	msg := center.Render(lipgloss.NewStyle().Foreground(ctpOverlay1).Render("not logged in"))
	hint := center.Render(
		lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).Render("ctrl+l") +
			lipgloss.NewStyle().Foreground(ctpOverlay1).Render(" to log in with Google"),
	)
	return "\n\n" + msg + "\n" + hint
}

// nowPlayingContent renders the dedicated Now Playing tab: a big thumbnail
// (blank lines reserved here, the actual image drawn separately via
// nowPlayingThumbCmd) followed by title/channel/duration/views and the
// full description.
func (m Model) nowPlayingContent(width int) string {
	if m.nowPlaying == nil || m.playingVid == nil || m.playingLocal {
		center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
		msg := center.Render(lipgloss.NewStyle().Foreground(ctpOverlay1).Render("nothing playing"))
		hint := center.Render(lipgloss.NewStyle().Foreground(ctpOverlay0).Render("enter on a video, anywhere, to play it"))
		return "\n\n" + msg + "\n" + hint
	}

	v := m.playingVid
	_, _, _, imgRows := m.nowPlayingImageRegion()

	var b strings.Builder
	b.WriteString("\n") // blank line before the image area
	haveImage := thumb.Supported() && m.previewedID == v.ID
	if haveImage {
		b.WriteString(strings.Repeat("\n", imgRows))
	} else {
		msg := "loading thumbnail…"
		if !thumb.Supported() {
			msg = "no preview — Kitty terminal required"
		}
		b.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Height(imgRows).
			Foreground(ctpOverlay0).Render(msg))
	}
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(ctpText).Bold(true).Render(truncate(v.Title, width)))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	details := "📺 " + v.Channel
	if v.Duration > 0 {
		details += "  ⏱  " + formatDuration(v.Duration)
	}
	if v.ViewCount > 0 {
		details += "  👁  " + formatViews(v.ViewCount) + " views"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ctpSubtext0).Render(truncate(details, width)))
	b.WriteString("\n\n")

	switch {
	case m.npDescriptionLoading:
		b.WriteString(lipgloss.NewStyle().Foreground(ctpOverlay0).Render("loading description…"))
	case m.npDescription != "":
		// Everything above this point (blank + image + blank + title +
		// divider + details + blank) is exactly 7 lines beyond imgRows —
		// clamp the description to whatever's left instead of letting an
		// unbounded description overflow past the panel's border.
		remaining := m.panelHeight() - 2 - (imgRows + 7)
		if remaining < 1 {
			remaining = 1
		}
		wrapped := lipgloss.NewStyle().Foreground(ctpSubtext1).Width(width).Render(m.npDescription)
		lines := strings.Split(wrapped, "\n")
		if len(lines) > remaining {
			lines = lines[:remaining]
			if remaining > 0 {
				lines[remaining-1] = truncate(lines[remaining-1], width-1) + "…"
			}
		}
		b.WriteString(strings.Join(lines, "\n"))
	default:
		b.WriteString(lipgloss.NewStyle().Foreground(ctpOverlay0).Render("no description available"))
	}

	return b.String()
}
// Catppuccin Mocha — the most-recognized "modern terminal" palette right
// now (huge ecosystem: editors, shells, Discord, hundreds of app ports).
// Named as semantic roles (not raw hex scattered through styles) so the
// whole theme can be swapped by editing one block.
var (
	ctpBase      = lipgloss.Color("#1e1e2e")
	ctpMantle    = lipgloss.Color("#181825")
	ctpText      = lipgloss.Color("#cdd6f4")
	ctpSubtext1  = lipgloss.Color("#bac2de")
	ctpSubtext0  = lipgloss.Color("#a6adc8")
	ctpOverlay1  = lipgloss.Color("#7f849c")
	ctpOverlay0  = lipgloss.Color("#6c7086")
	ctpSurface2  = lipgloss.Color("#585b70")
	ctpSurface1  = lipgloss.Color("#45475a")
	ctpMauve     = lipgloss.Color("#cba6f7")
	ctpPink      = lipgloss.Color("#f5c2e7")
	ctpRed       = lipgloss.Color("#f38ba8")
	ctpPeach     = lipgloss.Color("#fab387")
	ctpYellow    = lipgloss.Color("#f9e2af")
	ctpGreen     = lipgloss.Color("#a6e3a1")
	ctpBlue      = lipgloss.Color("#89b4fa")
	ctpLavender  = lipgloss.Color("#b4befe")
)

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(ctpMauve)
	subtitleStyle    = lipgloss.NewStyle().Foreground(ctpOverlay1).Italic(true)
	dividerStyle     = lipgloss.NewStyle().Foreground(ctpSurface1)
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Background(ctpMauve).Foreground(ctpBase).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(ctpOverlay0).Padding(0, 1)
	errorStyle       = lipgloss.NewStyle().Foreground(ctpRed).Bold(true)
	panelBorder      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ctpSurface1)
	previewLabel     = lipgloss.NewStyle().Foreground(ctpLavender).Bold(true)
	previewMutedText = lipgloss.NewStyle().Foreground(ctpOverlay0).Align(lipgloss.Center)
	// Footer rendered as a solid bar (like a vim statusline / k9s footer)
	// rather than plain text floating on the background — this is the
	// single highest-signal detail that makes a TUI look "finished".
	footerBarStyle  = lipgloss.NewStyle().Background(ctpMantle).Foreground(ctpSubtext0)
	footerModeNav   = lipgloss.NewStyle().Background(ctpBlue).Foreground(ctpBase).Bold(true).Padding(0, 1)
	footerModeInput = lipgloss.NewStyle().Background(ctpGreen).Foreground(ctpBase).Bold(true).Padding(0, 1)
)

// messages
type searchResultsMsg struct {
	results []ytdlp.Video
	err     error
}
type playbackStartedMsg struct {
	p   *player.Player
	err error
}
type thumbShownMsg struct{ id string }
type positionTickMsg struct{}
type nowPlayingTickMsg struct{}
type videoDescriptionMsg struct {
	videoID string // guards against a slow fetch for a video the person already skipped past
	desc    string
	err     error
}
type downloadsTickMsg struct{}
type thumbErrMsg struct{ err error }

type loginResultMsg struct{ err error }
type clientSecretsImportedMsg struct{ err error }
type authClientReadyMsg struct {
	client *http.Client
	err    error
}
type subsLoadedMsg struct {
	subs []ytapi.Subscription
	err  error
}
type playlistsLoadedMsg struct {
	playlists []ytapi.Playlist
	err       error
}
type playlistVideosLoadedMsg struct {
	videos []ytdlp.Video
	err    error
}
type channelVideosLoadedMsg struct {
	videos []ytdlp.Video
	err    error
}
type homeFeedLoadedMsg struct {
	videos []ytdlp.Video
	failed int
	err    error
}
type likedLoadedMsg struct {
	videos []ytdlp.Video
	err    error
}

// Model is the top-level Bubbletea model.
type Model struct {
	store *store.Store
	cfg   config.Config

	activeTab   tab
	inputMode   bool // true = typing in the search box; false = list navigation
	searchInput textinput.Model
	list        list.Model
	spin        spinner.Model
	loading      bool
	everSearched bool

	width, height int

	statusMsg string
	errMsg    string

	nowPlaying *player.Player
	playingVid *ytdlp.Video
	playingLocal bool
	audioOnly  bool

	npPos    float64
	npDur    float64
	npVol    float64
	npPaused bool

	npDescription        string
	npDescriptionLoading bool

	previewEnabled bool
	previewedID    string // avoid redrawing the same thumbnail every keystroke

	// searchResults holds the Search tab's items independently of the
	// shared list widget. Without this, switching to another tab (which
	// repoints the widget at that tab's items) and back would show
	// whatever tab you last visited instead of your search results —
	// there was nowhere else the results were being kept.
	searchResults []list.Item

	// --- Google login + real YouTube data (all optional; app works
	// entirely without login using the local store) ---
	loggedIn         bool
	authClient       *http.Client
	ytLoading        bool
	showLoginHelp    bool
	credentialStep   int // 0 = inactive, 1 = collecting Client ID, 2 = collecting Client Secret
	pendingClientID  string
	pathInput        textinput.Model

	realSubscriptions []list.Item // populated from the API once logged in
	browsingChannelID    string  // "" = showing the subscriptions list; else viewing this channel's uploads
	browsingChannelTitle string
	channelVideos         []list.Item

	playlists             []list.Item // top-level: the user's playlists
	browsingPlaylistID    string      // "" = showing the playlist list; else viewing this playlist's videos
	browsingPlaylistTitle string
	playlistVideos        []list.Item

	likedVideos []list.Item

	homeFeed []list.Item

	downloads *downloader.Manager
	offlineDownloads bool
	offlineFiles []library.File
	offlineDeletePending *library.File

	downloadPicker        bool
	downloadPickerVideo   *ytdlp.Video
	downloadPickerCursor  int
	downloadPickerQuality int
	downloadPickerSubs    bool
	downloadPickerThumb   bool
	downloadPickerCookies int // index into downloadCookieBrowserValues
}

// New builds the initial model.
