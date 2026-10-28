package ui

import "strings"

type paletteCommand struct {
	id string
	label string
	description string
}

func paletteCommands() []paletteCommand {
	return []paletteCommand{
		{id: "search", label: "Search YouTube", description: "Focus the search field"},
		{id: "home", label: "Open Home", description: "Switch to your Home feed"},
		{id: "refresh_home", label: "Refresh Home feed", description: "Fetch the latest Home feed"},
		{id: "subscriptions", label: "Open Subscriptions", description: "Browse subscribed channels"},
		{id: "playlists", label: "Open Playlists", description: "Browse your playlists"},
		{id: "liked", label: "Open Liked videos", description: "Browse liked videos"},
		{id: "watch_later", label: "Open Watch Later", description: "Open your saved queue"},
		{id: "history", label: "Open History", description: "Browse watch history"},
		{id: "downloads", label: "Open Downloads", description: "Show the live download queue"},
		{id: "offline", label: "Open offline files", description: "Browse downloaded media"},
		{id: "toggle_audio", label: "Toggle audio-only mode", description: "Change playback mode"},
		{id: "toggle_thumbnails", label: "Toggle thumbnail previews", description: "Enable or disable terminal previews"},
		{id: "toggle_autoplay", label: "Toggle autoplay next", description: "Save the autoplay preference"},
		{id: "clear_filters", label: "Clear library filters", description: "Clear text, status, and date filters"},
		{id: "help", label: "Open keyboard help", description: "Show all keybindings"},
	}
}

func filterPaletteCommands(commands []paletteCommand, query string) []paletteCommand {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" { return commands }
	filtered := make([]paletteCommand, 0, len(commands))
	for _, command := range commands {
		if strings.Contains(strings.ToLower(command.label+" "+command.description), query) {
			filtered = append(filtered, command)
		}
	}
	return filtered
}
