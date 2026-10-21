// Package ui implements the Bubbletea model/update/view for ytui.
//
// Layout follows the "persistent multi-panel" pattern used by lazygit,
// k9s, and btop: fixed-position bordered panels that fill the available
// terminal space, plus a full-width status bar — not a small floating
// card. Panels stay where the user expects them; only their content
// changes.
package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"ytui/internal/auth"
	"ytui/internal/config"
	"ytui/internal/downloader"
	"ytui/internal/library"
	"ytui/internal/player"
	"ytui/internal/store"
	"ytui/internal/thumb"
	"ytui/internal/ytapi"
	"ytui/internal/ytdlp"
)

type tab int

const (
	tabSearch tab = iota
	tabNowPlaying
	tabHome
	tabSubscriptions
	tabPlaylists
	tabLiked
	tabWatchLater
	tabHistory
	tabDownloads
)

var tabNames = []string{"Search", "Now Playing", "Home", "Subs", "Playlists", "Liked", "Watch Later", "History", "Downloads"}

// visibleTabs returns which tabs currently show in the tab bar. Now
// Playing only appears while something's actually playing — showing an
// empty "Now Playing" tab all the time would just be dead weight in the
// tab bar for the (usual) case where nothing's playing.
func (m Model) visibleTabs() []tab {
	out := make([]tab, 0, len(tabNames))
	for i := range tabNames {
		t := tab(i)
		if t == tabNowPlaying && m.nowPlaying == nil {
			continue
		}
		out = append(out, t)
	}
	return out
}

// nextTab/prevTab cycle among only the currently visible tabs, so tab/
// shift+tab never lands on a hidden Now Playing tab.
func (m Model) nextTab() tab {
	vis := m.visibleTabs()
	for i, t := range vis {
		if t == m.activeTab {
			return vis[(i+1)%len(vis)]
		}
	}
	return tabSearch
}

func (m Model) prevTab() tab {
	vis := m.visibleTabs()
	for i, t := range vis {
		if t == m.activeTab {
			return vis[(i-1+len(vis))%len(vis)]
		}
	}
	return tabSearch
}

// downloadQuality is one row of the ctrl+d quality picker.
type downloadQuality struct {
	Label        string
	Format       string
	ExtractAudio bool
	AudioFormat  string
}

var downloadQualities = []downloadQuality{
	{Label: "Best quality (video+audio)", Format: "bestvideo+bestaudio/best"},
	{Label: "1080p", Format: "bestvideo[height<=1080]+bestaudio/best[height<=1080]"},
	{Label: "720p", Format: "bestvideo[height<=720]+bestaudio/best[height<=720]"},
	{Label: "480p", Format: "bestvideo[height<=480]+bestaudio/best[height<=480]"},
	{Label: "Audio only (mp3)", ExtractAudio: true, AudioFormat: "mp3"},
	{Label: "Audio only (opus)", ExtractAudio: true, AudioFormat: "opus"},
}

// downloadCookieBrowsers cycles through yt-dlp's --cookies-from-browser
// choices. "" (None) is first/default — cookies are only needed as a
// workaround when YouTube's bot-check blocks a plain request, so we don't
// force them on by default. Index/label pairs kept parallel for the UI.
var downloadCookieBrowserValues = []string{"", "firefox", "chrome", "chromium", "brave", "edge"}
var downloadCookieBrowserLabelsBase = []string{"None", "Firefox", "Chrome", "Chromium", "Brave", "Edge"}

// browserConfigPaths are the standard Linux config directories for each
// browser — their presence is a reasonable signal the browser is actually
// installed (and has a profile yt-dlp could pull cookies from), without
// needing to shell out to anything.
var browserConfigPaths = map[string]string{
	"firefox":  ".mozilla/firefox",
	"chrome":   ".config/google-chrome",
	"chromium": ".config/chromium",
	"brave":    ".config/BraveSoftware/Brave-Browser",
	"edge":     ".config/microsoft-edge",
}

// detectDefaultCookieBrowser scans standard config paths and returns the
// index (into downloadCookieBrowserValues) of the first browser that looks
// installed, so the picker can default to it instead of forcing the person
// to know which value to pick. Falls back to 0 ("None") if nothing's found
// — cookies are only needed as a bot-check workaround, so "not needed by
// default" is the safe fallback, not a guess.
func detectDefaultCookieBrowser() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	for i, name := range downloadCookieBrowserValues {
		if name == "" {
			continue
		}
		rel, ok := browserConfigPaths[name]
		if !ok {
			continue
		}
		if info, err := os.Stat(filepath.Join(home, rel)); err == nil && info.IsDir() {
			return i
		}
	}
	return 0
}

// defaultCookieBrowserIndex prefers an explicit config.json override over
// auto-detection, since the person telling us directly beats us guessing.
func (m Model) defaultCookieBrowserIndex() int {
	if m.cfg.DefaultCookiesBrowser != "" {
		for i, v := range downloadCookieBrowserValues {
			if v == m.cfg.DefaultCookiesBrowser {
				return i
			}
		}
	}
	return detectDefaultCookieBrowser()
}

// defaultQualityIndex looks up a config-specified quality label (e.g.
// "1080p") among the picker's presets, falling back to "Best quality" (0)
// if unset or unrecognized.
func defaultQualityIndex(label string) int {
	if label == "" {
		return 0
	}
	for i, q := range downloadQualities {
		if q.Label == label {
			return i
		}
	}
	return 0
}

// downloadCookieBrowserLabels annotates whichever entry was auto-detected,
// so it's clear in the UI *why* that one's pre-selected rather than it
// looking like an arbitrary default.
func downloadCookieBrowserLabels(detectedIdx int) []string {
	labels := make([]string, len(downloadCookieBrowserLabelsBase))
	copy(labels, downloadCookieBrowserLabelsBase)
	if detectedIdx > 0 && detectedIdx < len(labels) {
		labels[detectedIdx] += " (detected)"
	}
	return labels
}

// downloadPickerRows returns the total navigable rows in the ctrl+d picker:
// one per quality preset, plus subtitles toggle, thumbnail toggle, cookies
// browser cycle, and the final "Start Download" confirm row.
func downloadPickerRows() int {
	return len(downloadQualities) + 4
}

// Header = title + tabs + divider. Footer = status line + mode bar.
const (
	headerLines = 3
)

// footerLines returns how many lines the footer currently occupies: the
// mode/help bar, plus either the richer 2-line Now Playing bar (title +
// progress) when something's playing, or a single status/error line
// otherwise. Dynamic rather than a fixed constant so panelHeight() shrinks
// the content panel appropriately instead of the Now Playing bar
// overflowing past what was budgeted for it.
func (m Model) footerLines() int {
	if m.nowPlaying != nil {
		return 3
	}
	return 2
}

// The preview panel's image now scales with the actual panel size (which
// itself scales with the terminal), within sane bounds so it doesn't
// vanish on a tiny terminal or become absurdly wide on an ultrawide one.
const (
	minPreviewPanelWidth = 30
	maxPreviewPanelWidth = 64
	previewWidthFraction = 0.32 // preview panel target width, as a fraction of total terminal width
)

const (
	defaultWidth  = 100
	defaultHeight = 30
)

// videoItem adapts ytdlp.Video to the list.Item interface bubbles/list needs.
// ytDelegate is a custom list.ItemDelegate replacing bubbles/list's
// DefaultDelegate. DefaultDelegate's internal title/description truncation
// assumes rune-count roughly equals terminal display width, which breaks
// down for scripts like Bengali, Devanagari, and Thai where combining
// marks and conjuncts make multiple Unicode codepoints render as a single,
// narrower glyph — that mismatch was producing garbled output (misaligned
// text, stray leftover border/padding characters) for that kind of
// content. This delegate does its own truncation via uniseg instead.
type ytDelegate struct{}

func (ytDelegate) Height() int                       { return 2 }
func (ytDelegate) Spacing() int                       { return 1 }
func (ytDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (ytDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	type titler interface{ Title() string }
	type describer interface{ Description() string }

	var title, desc string
	if t, ok := item.(titler); ok {
		title = t.Title()
	}
	if d, ok := item.(describer); ok {
		desc = d.Description()
	}

	innerWidth := m.Width() - 4 // selection bar + padding
	if innerWidth < 4 {
		innerWidth = 4
	}
	title = truncate(title, innerWidth)
	desc = truncate(desc, innerWidth)

	selected := index == m.Index()

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = lipgloss.NewStyle().Foreground(ctpMauve).Bold(true).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ctpMauve).PaddingLeft(1)
		descStyle = lipgloss.NewStyle().Foreground(ctpPink).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ctpMauve).PaddingLeft(1)
	} else {
		titleStyle = lipgloss.NewStyle().Foreground(ctpText).PaddingLeft(2)
		descStyle = lipgloss.NewStyle().Foreground(ctpSubtext0).PaddingLeft(2)
	}

	fmt.Fprint(w, titleStyle.Render(title)+"\n"+descStyle.Render(desc))
}

type videoItem struct{ v ytdlp.Video }

type localDownloadItem struct{ file library.File }

func (i localDownloadItem) Title() string { return i.file.Name }
func (i localDownloadItem) Description() string {
	return fmt.Sprintf("%s · modified %s", formatBytes(i.file.Size), i.file.ModTime.Format("Jan 2 15:04"))
}
func (i localDownloadItem) FilterValue() string { return i.file.Name }

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit { return fmt.Sprintf("%d B", n) }
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit { div *= unit; exp++ }
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (i videoItem) Title() string {
	return fmt.Sprintf("%s  [%s]", i.v.Title, formatDuration(i.v.Duration))
}
func (i videoItem) Description() string {
	return fmt.Sprintf("%s · %s views", i.v.Channel, formatViews(i.v.ViewCount))
}
func (i videoItem) FilterValue() string { return i.v.Title }

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatViews(v int64) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}

// truncate shortens s to at most width runes, appending an ellipsis if it
// had to cut anything. width <= 1 just returns "…".
// truncate shortens s to at most `width` terminal display columns,
// appending an ellipsis if it had to cut anything. Uses uniseg for actual
// grapheme-cluster-aware width, not a naive rune count — scripts like
// Bengali, Devanagari, and Thai combine multiple Unicode codepoints
// (base character + combining vowel signs / conjuncts) into a single
// visual glyph that occupies far fewer terminal columns than its rune
// count would suggest. Counting runes instead of display width is what
// caused garbled, misaligned rendering (stray leftover characters) for
// that kind of text. width <= 1 just returns "…".
func truncate(s string, width int) string {
	if uniseg.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	var b strings.Builder
	col := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		w := uniseg.StringWidth(gr.Str())
		if col+w > width-1 { // leave room for the ellipsis
			break
		}
		b.WriteString(gr.Str())
		col += w
	}
	return b.String() + "…"
}

// wrapTwoLines word-wraps s to fit width, returning at most 2 lines. If
// wrapping produces more than 2 lines, the second line is ellipsized so
// nothing overflows past it — used for the preview panel's video title,
// which previously got hard-truncated to a single line even when there
// was room to show more of it.
func wrapTwoLines(s string, width int) (line1, line2 string) {
	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > 0 {
		line1 = lines[0]
	}
	if len(lines) > 1 {
		line2 = lines[1]
	}
	if len(lines) > 2 {
		line2 = truncate(line2, width)
	}
	return line1, line2
}

// logoLines is a compact hand-built block-letter "YTUI" wordmark — the
// kind of first-screen personality popular TUIs (gum, opencode, glow) lead
// with instead of a bare input box. Each of the 4 letters is colored
// differently for a lightweight "gradient banner" effect.
var logoLines = []string{
	"█   █ █████ █   █ █████",
	" █ █    █   █   █   █  ",
	"  █     █   █   █   █  ",
	"  █     █   █   █   █  ",
	"  █     █    ███  █████",
}

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

func New(s *store.Store) Model {
	ti := textinput.New()
	ti.Placeholder = "Search YouTube... (press enter)"
	ti.Prompt = "🔍 "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ctpMauve)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ctpMauve)
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	pi := textinput.New()
	pi.Placeholder = "paste here…"
	pi.PromptStyle = lipgloss.NewStyle().Foreground(ctpGreen)
	pi.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
	pi.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
	pi.Cursor.Style = lipgloss.NewStyle().Foreground(ctpGreen)
	pi.CharLimit = 4000
	pi.Width = 60

	delegate := ytDelegate{}

	l := list.New(nil, delegate, defaultWidth, defaultHeight-headerLines-2)
	l.Title = "Results"
	l.Styles.Title = lipgloss.NewStyle().Background(ctpMauve).Foreground(ctpBase).Bold(true).Padding(0, 1)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we render our own help line, keyed to mode
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	cfg := config.Load()
	dl, _ := downloader.New(cfg.DownloadDir) // best-effort; nil is handled gracefully wherever it's used

	m := Model{
		store:          s,
		cfg:            cfg,
		activeTab:      tabSearch,
		inputMode:      true,
		searchInput:    ti,
		pathInput:      pi,
		list:           l,
		spin:           sp,
		width:          defaultWidth,
		height:         defaultHeight,
		previewEnabled: thumb.Supported(), // auto-on in Kitty; press 't' to disable if it glitches
		downloads:      dl,
		audioOnly:      cfg.DefaultAudioOnly,
	}
	m.applyListSize()
	return m
}

func (m Model) Init() tea.Cmd {
	if auth.HaveToken() {
		return tea.Batch(textinput.Blink, authClientCmd())
	}
	return textinput.Blink
}

func doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := ytdlp.Search(query, 25)
		return searchResultsMsg{results: results, err: err}
	}
}

func startPlayback(url string, audioOnly bool, resumePos float64) tea.Cmd {
	return func() tea.Msg {
		p, err := player.Play(url, audioOnly)
		if err == nil && p != nil && resumePos > 0 {
			// The IPC socket can become available before the media is loaded.
			// Wait for duration to become queryable before attempting the seek,
			// rather than relying on a fixed delay that varies by network speed.
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, durationErr := p.Duration(); durationErr == nil {
					if seekErr := p.SeekAbsolute(resumePos); seekErr == nil {
						break
					}
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return playbackStartedMsg{p: p, err: err}
	}
}

// positionTickCmd periodically saves the current playback position to
// history while a video is playing, not just on explicit stop/quit — so
// resume still works even if mpv gets closed some other way (its own 'q'
// key, closing its window, a crash) that we don't otherwise hear about.
func positionTickCmd() tea.Cmd {
	return tea.Tick(20*time.Second, func(time.Time) tea.Msg {
		return positionTickMsg{}
	})
}

// nowPlayingTickCmd drives the live Now Playing bar (position, duration,
// volume, pause state). Ticks much faster than positionTickCmd since a
// progress bar needs to feel live, whereas resume-position only needs
// occasional saves.
func nowPlayingTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return nowPlayingTickMsg{}
	})
}

func fetchDescriptionCmd(videoID, videoURL string) tea.Cmd {
	return func() tea.Msg {
		desc, err := ytdlp.GetDescription(videoURL)
		return videoDescriptionMsg{videoID: videoID, desc: desc, err: err}
	}
}

// downloadsTickCmd drives the live progress bar on the Downloads tab.
// Ticks fairly often (progress should feel live) but only while something
// is actually queued/downloading — see the downloadsTickMsg handler.
func downloadsTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return downloadsTickMsg{}
	})
}

func loginCmd() tea.Cmd {
	return func() tea.Msg {
		err := auth.Login(context.Background())
		return loginResultMsg{err: err}
	}
}

func importClientSecretsRawCmd(jsonText string) tea.Cmd {
	return func() tea.Msg {
		err := auth.ImportClientSecretsRaw(jsonText)
		return clientSecretsImportedMsg{err: err}
	}
}

func importClientCredentialsCmd(clientID, clientSecret string) tea.Cmd {
	return func() tea.Msg {
		err := auth.ImportClientCredentials(clientID, clientSecret)
		return clientSecretsImportedMsg{err: err}
	}
}

func authClientCmd() tea.Cmd {
	return func() tea.Msg {
		client, err := auth.Client(context.Background())
		return authClientReadyMsg{client: client, err: err}
	}
}

func fetchSubsCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		subs, err := ytapi.Subscriptions(client)
		return subsLoadedMsg{subs: subs, err: err}
	}
}

func fetchPlaylistsCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		pls, err := ytapi.Playlists(client)
		return playlistsLoadedMsg{playlists: pls, err: err}
	}
}

func fetchPlaylistVideosCmd(client *http.Client, playlistID string) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.PlaylistVideos(client, playlistID)
		return playlistVideosLoadedMsg{videos: videos, err: err}
	}
}

func fetchChannelVideosCmd(client *http.Client, channelID string) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.ChannelVideos(client, channelID)
		return channelVideosLoadedMsg{videos: videos, err: err}
	}
}

func fetchHomeFeedCmd(client *http.Client, subs []ytapi.Subscription) tea.Cmd {
	return func() tea.Msg {
		var err error
		if len(subs) == 0 {
			subs, err = ytapi.Subscriptions(client)
			if err != nil {
				return homeFeedLoadedMsg{err: err}
			}
		}
		if len(subs) == 0 {
			return homeFeedLoadedMsg{videos: nil}
		}
		videos, failed, err := ytapi.HomeFeed(client, subs, 5)
		return homeFeedLoadedMsg{videos: videos, failed: failed, err: err}
	}
}

func fetchLikedCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		videos, err := ytapi.LikedVideos(client)
		return likedLoadedMsg{videos: videos, err: err}
	}
}

// refreshPreview clears any currently-drawn thumbnail and redraws one for
// whatever's now selected, if applicable. Call this any time the *visible
// content* changes wholesale (switching tabs, drilling into/out of a
// playlist) — not just when moving the cursor within the same list.
// Without this, an old thumbnail (drawn outside Bubbletea's own render
// loop, so it isn't naturally cleared by a normal redraw) stays on screen
// after switching to a tab/state that has nothing to do with it.
func (m *Model) refreshPreview() tea.Cmd {
	_ = thumb.Clear()
	m.previewedID = ""
	if m.activeTab == tabNowPlaying {
		return m.nowPlayingThumbCmd()
	}
	if !m.previewVisible() {
		return nil
	}
	if v, ok := m.selectedVideo(); ok {
		return m.showThumbnailCmd(v)
	}
	return nil
}

// showThumbnailCmd downloads (if needed) and draws the thumbnail for v into
// the preview panel region. No-op if thumbnails aren't supported/enabled or
// the video has no thumbnail URL.
func (m Model) showThumbnailCmd(v ytdlp.Video) tea.Cmd {
	if !m.previewVisible() || v.Thumbnail == "" || v.ID == m.previewedID {
		return nil
	}
	col, row, cols, rows := m.previewImageRegion()
	return func() tea.Msg {
		path, err := thumb.Download(v.Thumbnail, v.ID)
		if err != nil {
			return thumbErrMsg{err}
		}
		_ = thumb.Clear()
		if err := thumb.Show(path, col, row, cols, rows); err != nil {
			return thumbErrMsg{err}
		}
		return thumbShownMsg{id: v.ID}
	}
}

// previewImageRegion computes the absolute terminal (col, row, cols, rows)
// the thumbnail image should draw into, inset a little from the preview
// panel's own borders.
// Preview panel content is a fixed vertical recipe: label, blank, image
// area, then a fixed-size info block. previewImageRegion and the panel
// renderer in View() must agree on these exactly, or the image ends up
// drawn over the label text instead of below it.
const (
	previewHeaderRows = 2 // "Preview" label + blank line
	previewFooterRows = 7 // blank separator, title (2 lines), divider, channel, duration, views
)

// nowPlayingInfoRows is how many lines the Now Playing tab's info block
// (below the thumbnail) reserves: blank, title (2 lines), divider,
// channel/duration/views, blank, description label, then description text
// itself gets whatever's left. Kept as a named constant — shared between
// the region math and the renderer — specifically because mismatched
// "how many lines does this reserve" constants have been the source of
// real layout bugs here before (image overlapping text, overflow).
const nowPlayingInfoRows = 7

func (m Model) currentListItems() []list.Item {
	switch m.activeTab {
	case tabNowPlaying:
		return nil
	case tabDownloads:
		if m.offlineDownloads {
			items := make([]list.Item, 0, len(m.offlineFiles))
			for _, file := range m.offlineFiles { items = append(items, localDownloadItem{file: file}) }
			return items
		}
		if m.downloads == nil {
			return nil
		}
		dls := m.downloads.List()
		items := make([]list.Item, 0, len(dls))
		for _, d := range dls {
			items = append(items, downloadItem{d})
		}
		return items
	case tabHome:
		return m.homeFeed
	case tabSubscriptions:
		if m.browsingChannelID != "" {
			return m.channelVideos
		}
		if m.loggedIn && len(m.realSubscriptions) > 0 {
			return m.realSubscriptions
		}
		items := make([]list.Item, 0, len(m.store.Subscriptions))
		for _, c := range m.store.Subscriptions {
			items = append(items, subItem{c})
		}
		return items
	case tabPlaylists:
		if m.browsingPlaylistID != "" {
			return m.playlistVideos
		}
		return m.playlists
	case tabLiked:
		return m.likedVideos
	case tabWatchLater:
		items := make([]list.Item, 0, len(m.store.WatchLater))
		for _, e := range m.store.WatchLater {
			items = append(items, watchLaterItem{e})
		}
		return items
	case tabHistory:
		items := make([]list.Item, 0, len(m.store.History))
		for _, e := range m.store.History {
			items = append(items, historyItem{e})
		}
		return items
	default: // tabSearch
		return m.searchResults
	}
}

// maybeFetchForTab kicks off a background fetch if the current tab needs
// YouTube API data that isn't cached yet. Safe to call on every tab
// switch — it no-ops if not logged in, already loading, or already cached.
func (m *Model) maybeFetchForTab() tea.Cmd {
	if !m.loggedIn || m.authClient == nil || m.ytLoading {
		return nil
	}
	switch m.activeTab {
	case tabHome:
		if len(m.homeFeed) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchHomeFeedCmd(m.authClient, nil)
	case tabSubscriptions:
		if m.browsingChannelID != "" {
			if len(m.channelVideos) > 0 {
				return nil
			}
			m.ytLoading = true
			return fetchChannelVideosCmd(m.authClient, m.browsingChannelID)
		}
		if len(m.realSubscriptions) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchSubsCmd(m.authClient)
	case tabPlaylists:
		if m.browsingPlaylistID != "" {
			if len(m.playlistVideos) > 0 {
				return nil
			}
			m.ytLoading = true
			return fetchPlaylistVideosCmd(m.authClient, m.browsingPlaylistID)
		}
		if len(m.playlists) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchPlaylistsCmd(m.authClient)
	case tabLiked:
		if len(m.likedVideos) > 0 {
			return nil
		}
		m.ytLoading = true
		return fetchLikedCmd(m.authClient)
	}
	return nil
}

type subItem struct{ c store.Channel }

func (i subItem) Title() string       { return i.c.Name }
func (i subItem) Description() string { return i.c.URL }
func (i subItem) FilterValue() string { return i.c.Name }

type realSubItem struct{ s ytapi.Subscription }

func (i realSubItem) Title() string       { return i.s.Title }
func (i realSubItem) Description() string { return "" }
func (i realSubItem) FilterValue() string { return i.s.Title }

type playlistItemUI struct{ p ytapi.Playlist }

func (i playlistItemUI) Title() string { return i.p.Title }
func (i playlistItemUI) Description() string {
	if i.p.ItemCount == 1 {
		return "1 video"
	}
	return fmt.Sprintf("%d videos", i.p.ItemCount)
}
func (i playlistItemUI) FilterValue() string { return i.p.Title }

type watchLaterItem struct{ e store.WatchLaterEntry }

func (i watchLaterItem) Title() string       { return i.e.Title }
func (i watchLaterItem) Description() string { return i.e.Channel }
func (i watchLaterItem) FilterValue() string { return i.e.Title }

type historyItem struct{ e store.HistoryEntry }

func (i historyItem) Title() string { return i.e.Title }
func (i historyItem) Description() string {
	return fmt.Sprintf("%s · watched %s", i.e.Channel, i.e.WatchedAt.Format("Jan 2 15:04"))
}
func (i historyItem) FilterValue() string { return i.e.Title }

type downloadItem struct{ d downloader.Download }

func (i downloadItem) Title() string { return i.d.Title }
func (i downloadItem) Description() string {
	q := i.d.Opts.QualityLabel
	if q != "" {
		q = q + " · "
	}
	switch i.d.Status {
	case downloader.StatusQueued:
		return q + "queued"
	case downloader.StatusDownloading:
		bar := progressBar(i.d.Progress, 20)
		return fmt.Sprintf("%s%s %5.1f%%  %s  ETA %s", q, bar, i.d.Progress, i.d.Speed, i.d.ETA)
	case downloader.StatusDone:
		return "✓ done — " + i.d.FilePath
	case downloader.StatusError:
		msg := "unknown error"
		if i.d.Err != nil {
			msg = i.d.Err.Error()
		}
		return "✗ error: " + msg
	case downloader.StatusCanceled:
		return "canceled"
	}
	return ""
}
func (i downloadItem) FilterValue() string { return i.d.Title }

// progressBar renders a simple block-character progress bar, e.g.
// "████████░░░░░░░░░░░░" for 40%.
func progressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width) * pct / 100)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applyListSize()
		_ = thumb.Clear()
		m.previewedID = ""
		// Bubbletea normally only redraws what changed, which can leave
		// stale characters behind at columns/rows a new, narrower frame no
		// longer writes to — a full clear on every resize avoids that.
		cmds := []tea.Cmd{tea.ClearScreen}
		if m.previewVisible() {
			if v, ok := m.selectedVideo(); ok {
				cmds = append(cmds, m.showThumbnailCmd(v))
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case searchResultsMsg:
		m.loading = false
		m.everSearched = true
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		items := make([]list.Item, 0, len(msg.results))
		for _, v := range msg.results {
			items = append(items, videoItem{v})
		}
		m.searchResults = items
		m.list.SetItems(items)
		m.applyListSize()
		m.statusMsg = fmt.Sprintf("%d results", len(items))
		var cmd tea.Cmd
		if len(items) > 0 {
			if vi, ok := items[0].(videoItem); ok {
				cmd = m.showThumbnailCmd(vi.v)
			}
		}
		return m, cmd

	case playbackStartedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.errMsg = ""
		}
		m.nowPlaying = msg.p
		m.npPos, m.npDur, m.npVol, m.npPaused = 0, 0, 0, false
		m.npDescription = ""
		if msg.err == nil && msg.p != nil {
			cmds := []tea.Cmd{tea.ClearScreen, positionTickCmd(), nowPlayingTickCmd()}
			if m.playingVid != nil {
				m.npDescriptionLoading = true
				cmds = append(cmds, fetchDescriptionCmd(m.playingVid.ID, m.playingVid.URL))
			}
			if m.activeTab == tabNowPlaying {
				cmds = append(cmds, m.refreshPreview())
			}
			return m, tea.Batch(cmds...)
		}
		return m, tea.ClearScreen

	case videoDescriptionMsg:
		m.npDescriptionLoading = false
		if msg.err != nil || m.playingVid == nil || m.playingVid.ID != msg.videoID {
			return m, nil // stale response for a video that's no longer playing, or fetch failed — not worth surfacing as an error
		}
		m.npDescription = msg.desc
		return m, nil

	case nowPlayingTickMsg:
		if m.nowPlaying == nil {
			return m, nil // playback ended/stopped since the last tick — don't reschedule
		}
		pos, err := m.nowPlaying.Position()
		if err != nil {
			// mpv is gone — the video finished, or it was closed some way
			// other than our own ctrl+x (its own quit key, closing its
			// window, a crash). Save the last UI-observed position before
			// clearing playback state; the fresh IPC query has already failed.
			m.saveResumePosition()
			return m.clearNowPlaying()
	}
		m.npPos = pos
		if dur, err := m.nowPlaying.Duration(); err == nil {
			m.npDur = dur
		}
		if vol, err := m.nowPlaying.Volume(); err == nil {
			m.npVol = vol
		}
		if paused, err := m.nowPlaying.IsPaused(); err == nil {
			m.npPaused = paused
		}
		return m, nowPlayingTickCmd()

	case positionTickMsg:
		if m.nowPlaying == nil {
			return m, nil // playback ended/stopped since the last tick — don't reschedule
		}
		m.saveResumePosition()
		return m, positionTickCmd()

	case downloadsTickMsg:
		if m.activeTab == tabDownloads {
			m.refreshList()
		}
		if m.downloads != nil && m.downloads.Active() {
			return m, downloadsTickCmd()
		}
		return m, nil

	case thumbShownMsg:
		m.previewedID = msg.id
		return m, nil
	case thumbErrMsg:
		m.errMsg = "thumbnail: " + msg.err.Error()
		return m, nil

	case loginResultMsg:
		if msg.err != nil {
			m.errMsg = "login: " + msg.err.Error()
			m.statusMsg = ""
			return m, nil
		}
		m.statusMsg = "verifying login…"
		return m, authClientCmd()

	case clientSecretsImportedMsg:
		m.credentialStep = 0
		m.pendingClientID = ""
		m.pathInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.showLoginHelp = false
		m.statusMsg = "client_secret.json imported ✓ — opening browser to log in…"
		return m, loginCmd()

	case authClientReadyMsg:
		if msg.err != nil {
			m.errMsg = "login: " + msg.err.Error()
			m.loggedIn = false
			m.authClient = nil
			return m, nil
		}
		m.loggedIn = true
		m.authClient = msg.client
		m.errMsg = ""
		return m, m.maybeFetchForTab()

	case subsLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "subscriptions: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.subs))
		for _, s := range msg.subs {
			items = append(items, realSubItem{s})
		}
		m.realSubscriptions = items
		if m.activeTab == tabSubscriptions {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case playlistsLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "playlists: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.playlists))
		for _, p := range msg.playlists {
			items = append(items, playlistItemUI{p})
		}
		m.playlists = items
		if m.activeTab == tabPlaylists && m.browsingPlaylistID == "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case playlistVideosLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "playlist videos: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.playlistVideos = items
		if m.activeTab == tabPlaylists && m.browsingPlaylistID != "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case channelVideosLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "channel uploads: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.channelVideos = items
		if m.activeTab == tabSubscriptions && m.browsingChannelID != "" {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case homeFeedLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "home feed: " + msg.err.Error()
			return m, nil
		}
		if msg.failed > 0 {
			m.statusMsg = fmt.Sprintf("home feed loaded with partial results: %d channel(s) failed", msg.failed)
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.homeFeed = items
		if m.activeTab == tabHome {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case likedLoadedMsg:
		m.ytLoading = false
		if msg.err != nil {
			m.errMsg = "liked videos: " + msg.err.Error()
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.videos))
		for _, v := range msg.videos {
			items = append(items, videoItem{v})
		}
		m.likedVideos = items
		if m.activeTab == tabLiked {
			m.refreshList()
			return m, m.refreshPreview()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) applyListSize() {
	listWidth, _ := m.paneWidths()
	listHeight := m.panelHeight() - 2 // panel top+bottom border
	if m.activeTab == tabSearch {
		listHeight -= 2 // search input line + divider line rendered above the list
	}
	m.list.SetSize(listWidth-2, listHeight)
}

// handleKey is the single source of truth for keyboard routing.
//
// There's no manual mode toggle to think about: on the Search tab, typing
// any letter/digit automatically starts (or continues) editing the query;
// pressing an arrow key or Enter-to-play automatically hands control back
// to list navigation. Because plain letters can now appear inside a typed
// query at any moment, all the old single-letter shortcuts (s/w/p/a/t/x/q)
// live on ctrl+ combos instead, so they never collide with what you're
// typing and work identically on every tab.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.offlineDeletePending != nil {
		pending := m.offlineDeletePending
		m.offlineDeletePending = nil
		switch key {
		case "y":
			if err := library.Delete(*pending); err != nil {
				m.errMsg = "delete download: " + err.Error()
			} else {
				m.errMsg = ""
				m.statusMsg = "deleted " + pending.Name
			}
			m.refreshList()
			return m, nil
		case "n", "esc":
			m.statusMsg = "delete canceled"
			return m, nil
		default:
			m.statusMsg = "delete canceled"
			return m, nil
		}
	}

	if key == "ctrl+c" {
		if m.nowPlaying != nil {
			m.saveResumePosition()
			_ = m.nowPlaying.Quit()
		}
		if m.previewedID != "" {
			_ = thumb.Clear()
			m.previewedID = ""
		}
		return m, tea.Quit
	}

	if m.downloadPicker {
		switch key {
		case "up":
			if m.downloadPickerCursor > 0 {
				m.downloadPickerCursor--
			}
			return m, nil
		case "down":
			if m.downloadPickerCursor < downloadPickerRows()-1 {
				m.downloadPickerCursor++
			}
			return m, nil
		case "enter", " ":
			qCount := len(downloadQualities)
			switch {
			case m.downloadPickerCursor < qCount:
				m.downloadPickerQuality = m.downloadPickerCursor
			case m.downloadPickerCursor == qCount:
				m.downloadPickerSubs = !m.downloadPickerSubs
			case m.downloadPickerCursor == qCount+1:
				m.downloadPickerThumb = !m.downloadPickerThumb
			case m.downloadPickerCursor == qCount+2:
				m.downloadPickerCookies = (m.downloadPickerCookies + 1) % len(downloadCookieBrowserValues)
			default: // "Start Download" row
				return m.startPickedDownload()
			}
			return m, nil
		case "esc":
			m.downloadPicker = false
			m.downloadPickerVideo = nil
			return m, nil
		}
		return m, nil // swallow anything else while the picker is open
	}

	if m.credentialStep > 0 {
		switch key {
		case "enter":
			raw := strings.TrimSpace(m.pathInput.Value())
			if raw == "" {
				return m, nil
			}
			if m.credentialStep == 1 {
				if strings.HasPrefix(raw, "{") {
					// Power-user shortcut: they pasted a whole downloaded
					// client_secret.json instead of just the Client ID.
					m.credentialStep = 0
					m.pathInput.Blur()
					m.pathInput.SetValue("")
					m.statusMsg = "importing…"
					return m, importClientSecretsRawCmd(raw)
				}
				m.pendingClientID = raw
				m.credentialStep = 2
				m.pathInput.SetValue("")
				m.pathInput.Placeholder = "paste your Client Secret…"
				return m, nil
			}
			// credentialStep == 2: raw is the Client Secret.
			clientID := m.pendingClientID
			m.credentialStep = 0
			m.pendingClientID = ""
			m.pathInput.Blur()
			m.pathInput.SetValue("")
			m.statusMsg = "importing…"
			return m, importClientCredentialsCmd(clientID, raw)
		case "esc":
			m.credentialStep = 0
			m.pendingClientID = ""
			m.pathInput.Blur()
			m.pathInput.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "tab":
		m.searchInput.Blur()
		m.inputMode = false
		m.activeTab = m.nextTab()
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
	case "shift+tab":
		m.searchInput.Blur()
		m.inputMode = false
		m.activeTab = m.prevTab()
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())

	case "/":
		if !m.inputMode {
			m.activeTab = tabSearch
			m.refreshList()
			m.inputMode = true
			m.searchInput.Focus()
			return m, tea.Batch(tea.ClearScreen, textinput.Blink, m.refreshPreview())
		}

	case "ctrl+l":
		if m.loggedIn {
			_ = auth.Logout()
			m.loggedIn = false
			m.authClient = nil
			m.realSubscriptions = nil
			m.playlists = nil
			m.playlistVideos = nil
			m.browsingPlaylistID = ""
			m.channelVideos = nil
			m.browsingChannelID = ""
			m.likedVideos = nil
			m.homeFeed = nil
			m.refreshList()
			m.statusMsg = "logged out"
			return m, nil
		}
		if !auth.HaveClientSecrets() {
			m.showLoginHelp = !m.showLoginHelp
			return m, nil
		}
		m.showLoginHelp = false
		m.statusMsg = "opening browser to log in with Google…"
		m.errMsg = ""
		return m, loginCmd()

	case "up", "down", "pgup", "pgdown", "home", "end":
		// Arrow-family keys always mean "navigate the list" — release the
		// search box automatically if it was focused, no Esc needed.
		if m.inputMode {
			m.inputMode = false
			m.searchInput.Blur()
		}
		return m.moveSelection(msg)

	case "enter":
		if m.activeTab == tabDownloads && m.offlineDownloads {
			if item, ok := m.list.SelectedItem().(localDownloadItem); ok {
				return m.playLocalFile(item.file)
			}
			return m, nil
		}
		if m.inputMode {
			query := strings.TrimSpace(m.searchInput.Value())
			if query == "" {
				return m, nil
			}
			m.loading = true
			m.inputMode = false
			m.searchInput.Blur()
			return m, doSearch(query)
		}
		if m.showLoginHelp && !auth.HaveClientSecrets() {
			m.credentialStep = 1
			m.pathInput.Placeholder = "paste your Client ID…"
			m.pathInput.Focus()
			m.pathInput.SetValue("")
			m.errMsg = ""
			return m, textinput.Blink
		}
		if m.activeTab == tabPlaylists && m.browsingPlaylistID == "" {
			if pl, ok := m.list.SelectedItem().(playlistItemUI); ok {
				m.browsingPlaylistID = pl.p.ID
				m.browsingPlaylistTitle = pl.p.Title
				m.playlistVideos = nil
				m.refreshList()
				return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
			}
			return m, nil
		}
		if m.activeTab == tabSubscriptions && m.browsingChannelID == "" {
			if s, ok := m.list.SelectedItem().(realSubItem); ok {
				m.browsingChannelID = s.s.ChannelID
				m.browsingChannelTitle = s.s.Title
				m.channelVideos = nil
				m.refreshList()
				return m, tea.Batch(tea.ClearScreen, m.maybeFetchForTab(), m.refreshPreview())
			}
			// Local (non-API) subscriptions don't carry a real channel ID,
			// so there's nothing to browse into — Enter is a no-op there
			// rather than a broken drill-in.
			return m, nil
		}
		return m.playSelected()

	case "esc":
		if m.inputMode {
			m.inputMode = false
			m.searchInput.Blur()
			return m, nil
		}
		if m.showLoginHelp {
			m.showLoginHelp = false
			return m, nil
		}
		if m.activeTab == tabPlaylists && m.browsingPlaylistID != "" {
			m.browsingPlaylistID = ""
			m.playlistVideos = nil
			m.refreshList()
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		}
		if m.activeTab == tabSubscriptions && m.browsingChannelID != "" {
			m.browsingChannelID = ""
			m.channelVideos = nil
			m.refreshList()
			return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
		}
		return m, nil

	case "ctrl+t":
		m.previewEnabled = !m.previewEnabled
		if m.previewEnabled {
			if thumb.Supported() {
				m.statusMsg = "thumbnail preview ON (experimental — ctrl+t again if the screen glitches)"
			} else {
				m.previewEnabled = false
				m.statusMsg = "this terminal doesn't support Kitty graphics — preview stays off"
			}
		} else {
			m.statusMsg = "thumbnail preview OFF"
			_ = thumb.Clear()
			m.previewedID = ""
		}
		m.applyListSize()
		return m, nil

	case "ctrl+a":
		m.audioOnly = !m.audioOnly
		if m.audioOnly {
			m.statusMsg = "audio-only mode ON"
		} else {
			m.statusMsg = "audio-only mode OFF"
		}
		return m, nil

	case "ctrl+s":
		return m.subscribeToSelected()
	case "ctrl+w":
		return m.addSelectedToWatchLater()
	case "ctrl+o":
		if m.activeTab == tabDownloads {
			m.offlineDownloads = !m.offlineDownloads
			m.errMsg = ""
			m.refreshList()
			if m.offlineDownloads {
				m.statusMsg = fmt.Sprintf("offline downloads: %d media file(s)", len(m.offlineFiles))
			} else {
				m.statusMsg = "download queue"
			}
		}
		return m, nil

	case "delete":
		if m.activeTab == tabDownloads && m.offlineDownloads {
			if item, ok := m.list.SelectedItem().(localDownloadItem); ok {
				file := item.file
				m.offlineDeletePending = &file
				m.statusMsg = "delete " + file.Name + "? press y to confirm, any other key to cancel"
			}
		}
		return m, nil

	case "ctrl+d":
		if m.activeTab == tabDownloads {
			if m.offlineDownloads { return m, nil }
			return m.cancelSelectedDownload()
		}
		if v, ok := m.selectedVideo(); ok {
			m.downloadPicker = true
			m.downloadPickerVideo = &v
			m.downloadPickerCursor = 0
			m.downloadPickerQuality = defaultQualityIndex(m.cfg.DefaultQuality)
			m.downloadPickerSubs = false
			m.downloadPickerThumb = false
			m.downloadPickerCookies = m.defaultCookieBrowserIndex()
		}
		return m, nil

	case "ctrl+p":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.TogglePause()
		}
		return m, nil
	case "[":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.SeekRelative(-10)
		}
		return m, nil
	case "]":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.SeekRelative(10)
		}
		return m, nil
	case "-":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.VolumeAdjust(-5)
		}
		return m, nil
	case "=":
		if m.nowPlaying != nil {
			_ = m.nowPlaying.VolumeAdjust(5)
		}
		return m, nil
	case "ctrl+x":
		if m.nowPlaying != nil {
			m.saveResumePosition()
			_ = m.nowPlaying.Quit()
			m.statusMsg = "playback stopped"
			return m.clearNowPlaying()
		}
		return m, nil
	}

	// Already typing — everything else (letters, digits, backspace,
	// left/right cursor movement, punctuation) goes to the text box.
	if m.inputMode {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	// Not typing yet: a plain letter/digit/symbol on the Search tab means
	// "start a query" — auto-focus the box and forward this keystroke as
	// its first character, instead of requiring an explicit "/" first.
	if m.activeTab == tabSearch && msg.Type == tea.KeyRunes {
		m.inputMode = true
		m.searchInput.Focus()
		m.searchInput.SetValue("")
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, tea.Batch(cmd, textinput.Blink)
	}

	// Everything else — j/k, g/G, etc. — goes to the list. On the Search
	// tab, letters are claimed by typing above, so only non-letter keys
	// (already handled explicitly, e.g. arrows) reach here; on the other
	// tabs, where there's no text box to worry about, this restores full
	// list-widget navigation (j/k included).
	return m.moveSelection(msg)
}

// moveSelection forwards a navigation key to the list widget and, if the
// selection actually changed, kicks off a thumbnail redraw.
func (m Model) moveSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	prevIndex := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if m.list.Index() != prevIndex {
		if v, ok := m.selectedVideo(); ok {
			if thumbCmd := m.showThumbnailCmd(v); thumbCmd != nil {
				return m, tea.Batch(cmd, thumbCmd)
			}
		}
	}
	return m, cmd
}

func (m *Model) refreshList() {
	if m.activeTab == tabDownloads && m.offlineDownloads {
		files, err := library.Scan(m.downloadDirectory())
		if err != nil {
			m.offlineFiles = nil
			m.errMsg = "offline downloads: " + err.Error()
		} else {
			m.offlineFiles = files
			m.errMsg = ""
		}
	}
	m.list.SetItems(m.currentListItems())
	switch {
	case m.activeTab == tabDownloads && m.offlineDownloads:
		m.list.Title = "Offline Downloads"
	case m.activeTab == tabPlaylists && m.browsingPlaylistID != "":
		m.list.Title = m.browsingPlaylistTitle
	case m.activeTab == tabSubscriptions && m.browsingChannelID != "":
		m.list.Title = m.browsingChannelTitle
	default:
		m.list.Title = tabNames[m.activeTab]
	}
	m.applyListSize()
}

// contextualHelp builds the BROWSE-mode footer hint, showing only
// keybinds that would actually do something right now — e.g. ^s (subscribe)
// only appears when a video is actually selected, not on every screen
// regardless of whether there's anything to subscribe to.
func (m Model) contextualHelp() string {
	parts := []string{"tab: switch view"}
	if m.activeTab == tabSearch {
		parts = append(parts, "type to search")
	}

	hasItems := len(m.list.Items()) > 0
	if hasItems {
		parts = append(parts, "↑↓: move")
	}

	if hasItems {
		switch m.list.SelectedItem().(type) {
		case playlistItemUI:
			parts = append(parts, "enter: open playlist")
		case realSubItem:
			parts = append(parts, "enter: open channel")
		default:
			if _, ok := m.selectedVideo(); ok {
				parts = append(parts, "enter: play")
			}
		}
	}

	if _, ok := m.selectedVideo(); ok {
		parts = append(parts, "^s: sub", "^w: later", "^d: download")
	}
	if m.activeTab == tabDownloads {
		if m.offlineDownloads {
			if hasItems { parts = append(parts, "enter: play", "delete: remove (confirm with y)") }
			parts = append(parts, "^o: queue")
		} else {
			parts = append(parts, "^o: offline files")
		}
	}
	if hasItems && m.activeTab == tabDownloads && !m.offlineDownloads {
		if item, ok := m.list.SelectedItem().(downloadItem); ok &&
			(item.d.Status == downloader.StatusQueued || item.d.Status == downloader.StatusDownloading) {
			parts = append(parts, "^d: cancel")
		}
	}

	parts = append(parts, "^a: audio")

	if hasItems && thumb.Supported() {
		parts = append(parts, "^t: thumbs")
	}

	if !m.loggedIn {
		parts = append(parts, "^l: login")
	}
	parts = append(parts, "^c: quit")

	return strings.Join(parts, " · ")
}

func (m Model) downloadDirectory() string {
	if m.downloads != nil { return m.downloads.DestDir() }
	if m.cfg.DownloadDir != "" { return m.cfg.DownloadDir }
	home, err := os.UserHomeDir()
	if err != nil { return "" }
	return filepath.Join(home, "Downloads", "ytui")
}

func (m Model) selectedVideo() (ytdlp.Video, bool) {
	item := m.list.SelectedItem()
	if item == nil {
		return ytdlp.Video{}, false
	}
	switch it := item.(type) {
	case videoItem:
		return it.v, true
	case watchLaterItem:
		return ytdlp.Video{ID: it.e.VideoID, Title: it.e.Title, Channel: it.e.Channel, URL: it.e.URL,
			Thumbnail: thumbnailFor(it.e.VideoID, it.e.Thumbnail)}, true
	case historyItem:
		return ytdlp.Video{ID: it.e.VideoID, Title: it.e.Title, Channel: it.e.Channel, URL: it.e.URL,
			Thumbnail: thumbnailFor(it.e.VideoID, it.e.Thumbnail)}, true
	}
	return ytdlp.Video{}, false
}

// thumbnailFor returns stored if it's non-empty, otherwise derives
// YouTube's standard thumbnail CDN URL directly from the video ID — this
// works for every video regardless of when the entry was saved, so older
// History/Watch Later entries (from before thumbnails were stored) still
// get a preview without needing any data migration.
func thumbnailFor(videoID, stored string) string {
	if stored != "" {
		return stored
	}
	if videoID == "" {
		return ""
	}
	return "https://i.ytimg.com/vi/" + videoID + "/mqdefault.jpg"
}

// saveResumePosition asks mpv for the current playback position and
// updates the video's history entry with it — so next time it's played,
// we can offer to resume where you left off. Best-effort: if mpv doesn't
// respond in time (e.g. it's already exiting), this just silently no-ops
// rather than blocking the quit/stop action.
func (m Model) saveResumePosition() {
	if m.nowPlaying == nil || m.playingVid == nil {
		return
	}
	pos, err := m.nowPlaying.Position()
	if err != nil || pos <= 0 {
		// mpv may already be exiting. Preserve the most recent position
		// observed by the UI rather than discarding all progress since the
		// previous periodic save.
		pos = m.npPos
	}
	if pos <= 0 {
		return
	}
	v := m.playingVid
	_ = m.store.AddHistory(store.HistoryEntry{
		VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL,
		Thumbnail: v.Thumbnail, PositionS: pos,
	})
}

func (m Model) playLocalFile(file library.File) (tea.Model, tea.Cmd) {
	if m.nowPlaying != nil {
		m.saveResumePosition()
		_ = m.nowPlaying.Quit()
	}
	m.playingLocal = true
	m.playingVid = &ytdlp.Video{ID: "local:" + file.Path, Title: file.Name, Channel: "Downloaded file", URL: file.Path}
	m.statusMsg = "starting mpv: " + file.Name
	return m, startPlayback(file.Path, m.audioOnly, 0)
}

func (m Model) playSelected() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	if m.nowPlaying != nil {
		m.saveResumePosition()
		_ = m.nowPlaying.Quit()
	}
	m.playingLocal = false
	m.playingVid = &v
	resumePos, resuming := m.store.FindHistoryPosition(v.ID)
	if resuming {
		m.statusMsg = fmt.Sprintf("starting mpv (resuming from %s): %s", formatDuration(resumePos), v.Title)
	} else {
		resumePos = 0
		m.statusMsg = "starting mpv: " + v.Title
	}
	_ = m.store.AddHistory(store.HistoryEntry{VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL, Thumbnail: v.Thumbnail, PositionS: resumePos})
	return m, startPlayback(v.URL, m.audioOnly, resumePos)
}

// clearNowPlaying resets all Now Playing state — used both when we stop
// playback ourselves (ctrl+x) and when the 1s poll detects mpv is gone
// (video ended, closed some other way). If the person is currently
// viewing the Now Playing tab, navigates them away from it since it'd
// otherwise show stale/empty content while still being "selected".
func (m Model) clearNowPlaying() (tea.Model, tea.Cmd) {
	m.nowPlaying = nil
	m.playingVid = nil
	m.playingLocal = false
	m.npPos, m.npDur, m.npVol, m.npPaused = 0, 0, 0, false
	m.npDescription = ""
	m.npDescriptionLoading = false
	if m.activeTab == tabNowPlaying {
		m.activeTab = tabSearch
		m.refreshList()
		return m, tea.Batch(tea.ClearScreen, m.refreshPreview())
	}
	return m, tea.ClearScreen
}

func (m Model) subscribeToSelected() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok || v.Channel == "" {
		return m, nil
	}
	if v.ChannelID == "" && v.ChannelURL == "" {
		m.statusMsg = "can't subscribe: channel identifier unavailable in search metadata"
		return m, nil
	}
	id := v.ChannelID
	if id == "" {
		id = v.ChannelURL
	}
	if err := m.store.Subscribe(store.Channel{ID: id, Name: v.Channel, URL: v.ChannelURL}); err != nil {
		m.statusMsg = "failed to subscribe: " + err.Error()
		return m, nil
	}
	m.statusMsg = "subscribed to " + v.Channel
	return m, nil
}

func (m Model) cancelSelectedDownload() (tea.Model, tea.Cmd) {
	if item, ok := m.list.SelectedItem().(downloadItem); ok {
		if m.downloads != nil && (item.d.Status == downloader.StatusQueued || item.d.Status == downloader.StatusDownloading) {
			m.downloads.Cancel(item.d.VideoID)
			m.statusMsg = "canceled: " + item.d.Title
		}
	}
	return m, nil
}

// startPickedDownload enqueues the download with whatever quality/toggle
// choices are currently set in the picker, then closes it.
func (m Model) startPickedDownload() (tea.Model, tea.Cmd) {
	v := m.downloadPickerVideo
	m.downloadPicker = false
	m.downloadPickerVideo = nil
	if v == nil {
		return m, nil
	}
	if m.downloads == nil {
		m.statusMsg = "downloads unavailable — couldn't create ~/Downloads/ytui"
		return m, nil
	}
	q := downloadQualities[m.downloadPickerQuality]
	opts := downloader.Options{
		Format:             q.Format,
		ExtractAudio:       q.ExtractAudio,
		AudioFormat:        q.AudioFormat,
		WriteSubs:          m.downloadPickerSubs,
		EmbedThumbnail:     m.downloadPickerThumb,
		CookiesFromBrowser: downloadCookieBrowserValues[m.downloadPickerCookies],
		QualityLabel:       q.Label,
	}
	wasActive := m.downloads.Active()
	m.downloads.Enqueue(v.ID, v.Title, v.Channel, v.URL, opts)
	m.statusMsg = "queued (" + q.Label + "): " + v.Title
	if !wasActive {
		return m, downloadsTickCmd() // start the progress-refresh loop
	}
	return m, nil
}

func (m Model) addSelectedToWatchLater() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	_ = m.store.AddWatchLater(store.WatchLaterEntry{VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL, Thumbnail: v.Thumbnail})
	m.statusMsg = "added to watch later: " + v.Title
	return m, nil
}
