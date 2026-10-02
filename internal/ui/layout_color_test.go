package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

// В реальном терминале цвета включены, а в go test — нет, и без цветов
// lipgloss скрывал переполнение таблицы. Включаем цвета принудительно и
// проверяем, что ни одна строка экрана не шире терминала.
func TestView_NoLineWiderThanTerminal_WithColors(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	for _, w := range []int{100, 120, 180, 181} {
		m := newTestModel("h", 1, "t", []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"})
		m, _ = update(m, tea.WindowSizeMsg{Width: w, Height: 50})
		m, _ = update(m, statusMsg(statusclient.Status{
			Type: "status",
			Symbols: map[string]statusclient.SymbolStatus{
				"BTC_USDT": {SnapshotsSinceStart: 698311, LastSnapshotAgeSecs: 0.1},
			},
		}))
		for i, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got != w {
				t.Fatalf("width=%d: строка %d шириной %d, want %d", w, i, got, w)
			}
		}
	}
}
