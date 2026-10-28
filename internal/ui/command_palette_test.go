package ui

import (
	"strings"
	"testing"
)

func TestFilterPaletteCommandsMatchesLabelsAndDescriptions(t *testing.T) {
	commands := paletteCommands()
	got := filterPaletteCommands(commands, "  DOWNLOAD ")
	if len(got) == 0 {
		t.Fatal("filterPaletteCommands() returned no download commands")
	}
	found := false
	for _, command := range got {
		if command.id == "downloads" { found = true }
		if !strings.Contains(strings.ToLower(command.label+" "+command.description), "download") {
			t.Fatalf("non-matching command returned: %#v", command)
		}
	}
	if !found { t.Fatal("filtered commands do not include Open Downloads") }
}

func TestFilterPaletteCommandsEmptyQueryReturnsAll(t *testing.T) {
	all := paletteCommands()
	got := filterPaletteCommands(all, "  ")
	if len(got) != len(all) { t.Fatalf("empty query returned %d commands, want %d", len(got), len(all)) }
}

func TestFilterPaletteCommandsNoMatches(t *testing.T) {
	if got := filterPaletteCommands(paletteCommands(), "definitely-no-command"); len(got) != 0 {
		t.Fatalf("unmatched query returned %d commands, want none", len(got))
	}
}
