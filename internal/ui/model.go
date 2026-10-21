// Package ui implements the Bubbletea model/update/view for ytui.
//
// Layout follows the "persistent multi-panel" pattern used by lazygit,
// k9s, and btop: fixed-position bordered panels that fill the available
// terminal space, plus a full-width status bar — not a small floating
// card. Panels stay where the user expects them; only their content
// changes.
package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
