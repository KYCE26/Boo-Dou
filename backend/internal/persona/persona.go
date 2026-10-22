package persona

import (
	_ "embed"
	"fmt"
	"strings"
	"time"
	// Windows has no system tzdata; embedding it keeps Asia/Jakarta loadable everywhere.
	_ "time/tzdata"

	"boodou/backend/internal/llm"
)

//go:embed boo_dou.md
var template string

var jakarta = mustLoad("Asia/Jakarta")

var days = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

var months = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("persona: zona waktu %s tidak tersedia: %v", name, err))
	}
	return loc
}

// FormatDate renders t in Jakarta time as "Kamis, 22 Oktober 2026". Go's
// layout strings only know English names, hence the lookup tables.
func FormatDate(t time.Time) string {
	t = t.In(jakarta)
	return fmt.Sprintf("%s, %d %s %d", days[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
}

// SystemPrompt is rebuilt per request so {{DATE}} stays correct on a server
// that runs past midnight.
func SystemPrompt(model string, now time.Time) string {
	return strings.NewReplacer(
		"{{MODEL}}", model,
		"{{DATE}}", FormatDate(now),
	).Replace(strings.TrimSpace(template))
}

// Apply puts the persona first and drops any client "system" messages, so the
// browser cannot override who Boo Dou is.
func Apply(model string, now time.Time, messages []llm.Message) []llm.Message {
	out := make([]llm.Message, 0, len(messages)+1)
	out = append(out, llm.Message{Role: "system", Content: SystemPrompt(model, now)})
	for _, m := range messages {
		if m.Role != "system" {
			out = append(out, m)
		}
	}
	return out
}
