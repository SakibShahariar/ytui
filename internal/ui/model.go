// Package ui implements the Bubbletea model/update/view for ytui.
//
// Layout follows the "persistent multi-panel" pattern used by lazygit,
// k9s, and btop: fixed-position bordered panels that fill the available
// terminal space, plus a full-width status bar — not a small floating
// card. Panels stay where the user expects them; only their content
// changes.
package ui

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func (m Model) addSelectedToWatchLater() (tea.Model, tea.Cmd) {
	v, ok := m.selectedVideo()
	if !ok {
		return m, nil
	}
	_ = m.store.AddWatchLater(store.WatchLaterEntry{VideoID: v.ID, Title: v.Title, Channel: v.Channel, URL: v.URL, Thumbnail: v.Thumbnail})
	m.statusMsg = "added to watch later: " + v.Title
	return m, nil
}
