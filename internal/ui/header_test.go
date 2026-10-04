package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// Шапка не должна переносить блоки на лишние строки на типичных
// ширинах терминала, а подпись "history" — исчезать на узких.
func TestRenderHeader_FitsHeightAtCommonWidths(t *testing.T) {
	for _, w := range []int{110, 120, 200} {
		h := renderHeader(1234567, w)
		if got := lipgloss.Height(h); got != headerHeight {
			t.Errorf("width=%d: высота шапки %d, want %d\n%s", w, got, headerHeight, h)
		}
		if got := lipgloss.Width(h); got != w {
			t.Errorf("width=%d: ширина шапки %d, want %d", w, got, w)
		}
	}
}

func TestRenderHeader_TaglineHiddenWhenNarrow(t *testing.T) {
	if !strings.Contains(renderHeader(1234567, 120), "history") {
		t.Error("на 120 колонках подпись history должна быть видна")
	}
	if strings.Contains(renderHeader(1234567, 100), "history") {
		t.Error("на 100 колонках подпись history должна быть скрыта")
	}
}
