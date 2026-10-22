package store

import "strings"

const titleMaxRunes = 60

// TitleFrom builds a conversation title from the first user message without
// asking the model, which would cost a request per conversation.
func TitleFrom(text string, hasImages bool) string {
	flat := strings.Join(strings.Fields(text), " ")
	if flat == "" {
		if hasImages {
			return "Gambar"
		}
		return "Percakapan baru"
	}
	runes := []rune(flat)
	if len(runes) <= titleMaxRunes {
		return flat
	}
	// Cutting exactly at a space is already a word boundary.
	if runes[titleMaxRunes] == ' ' {
		return string(runes[:titleMaxRunes])
	}
	cut := string(runes[:titleMaxRunes])
	if i := strings.LastIndex(cut, " "); i > 0 {
		return cut[:i]
	}
	// One very long word: a hard cut beats an empty title.
	return cut
}
