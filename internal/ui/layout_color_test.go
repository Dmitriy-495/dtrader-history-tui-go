package ui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

// Ни одна строка экрана не должна отличаться по ширине от терминала, а
// строки тела (между шапкой и футером) — быть шириной ровно в терминал и
// БЕЗ добивки пробелами: lipgloss.JoinVertical выравнивает короткие
// строки пробелами до самой широкой, и это маскирует разъезд рамок
// (в lipgloss v2 Width/Height включают рамку — ошибка в арифметике
// "width-2" проявляется именно так).
func TestView_BodyLinesMatchTerminalWidth(t *testing.T) {
	for _, w := range []int{100, 120, 180, 181} {
		m := newTestModel("h", 1, "t", []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"})
		m, _ = update(m, tea.WindowSizeMsg{Width: w, Height: 50})
		m, _ = update(m, statusMsg(statusclient.Status{
			Type: "status",
			Symbols: map[string]statusclient.SymbolStatus{
				"BTC_USDT": {SnapshotsSinceStart: 698311, LastSnapshotAgeSecs: 0.1},
			},
		}))
		lines := strings.Split(m.render(), "\n")
		for i, line := range lines {
			if got := lipgloss.Width(line); got != w {
				t.Fatalf("width=%d: строка %d шириной %d, want %d", w, i, got, w)
			}
		}
		const footerHeight = 3
		for i := headerHeightAt(w); i < len(lines)-footerHeight; i++ {
			if got := lipgloss.Width(strings.TrimRight(lines[i], " ")); got != w {
				t.Fatalf("width=%d: строка тела %d без добивки пробелами шириной %d, want %d", w, i, got, w)
			}
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
