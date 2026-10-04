package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

// После получения статуса строки данных должны реально попасть в
// отрисованный экран, а не только в модель таблицы. В bubbles v2 при
// нулевой ширине viewport таблицы данные не рисовались (виден был лишь
// заголовок), и ни один другой тест этого не замечал.
func TestView_ShowsSymbolRowsAfterStatus(t *testing.T) {
	m := newTestModel("h", 1, "t", []string{"BTC_USDT", "SOL_USDT"})
	m, _ = update(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m, _ = update(m, statusMsg(statusclient.Status{
		Type: "status",
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 698311, LastSnapshotAgeSecs: 0.1},
			"SOL_USDT": {SnapshotsSinceStart: 651018, LastSnapshotAgeSecs: 0.2},
		},
	}))

	out := m.render()
	for _, want := range []string{"BTC_USDT", "698311", "SOL_USDT", "651018"} {
		if !strings.Contains(out, want) {
			t.Errorf("в отрисованном экране нет %q", want)
		}
	}
}
