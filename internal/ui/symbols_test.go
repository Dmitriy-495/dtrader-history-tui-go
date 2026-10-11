package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

func TestTickerOf(t *testing.T) {
	for in, want := range map[string]string{"BTC_USDT": "BTC", "AVAX_USDT": "AVAX", "1INCH_USDT": "1INCH", "ETH": "ETH", "_X": "_X"} {
		if got := tickerOf(in); got != want {
			t.Errorf("tickerOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func tallModel(t *testing.T, font banner.Font, symbols []string, w, h int) Model {
	t.Helper()
	m := newTestModel("h", 1, "t", symbols).WithTickerFont(font)
	u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = u.(Model)
	st := statusclient.Status{Type: "status", UptimeSeconds: 10, Symbols: map[string]statusclient.SymbolStatus{}}
	for _, s := range symbols {
		st.Symbols[s] = statusclient.SymbolStatus{SnapshotsSinceStart: 123456, LastSnapshotAgeSecs: 0.4}
	}
	u, _ = m.Update(statusMsg(st))
	return u.(Model)
}

// Левая панель в режиме крупных тикеров ровно width × height на любой
// ширине и для любого шрифта (включая откат на узкой панели).
func TestTallSymbols_ExactSizeAcrossWidthsAndFonts(t *testing.T) {
	syms := []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"}
	for _, f := range []banner.Font{banner.FontMini, banner.FontSmall, banner.FontStandard, banner.FontBanner3Compact, banner.FontBanner3Mini} {
		for w := 60; w <= 260; w += 7 {
			m := tallModel(t, f, syms, w, 40)
			cw, ch := m.contentWidth()-2, m.bodyHeight()-2
			s := m.renderSymbolsTall(cw, ch)
			if got := lipgloss.Width(s); got != cw {
				t.Fatalf("%s w=%d: ширина %d, want %d", f, w, got, cw)
			}
			if got := lipgloss.Height(s); got != ch {
				t.Fatalf("%s w=%d: высота %d, want %d", f, w, got, ch)
			}
		}
	}
}

// Экран целиком: каждая строка тела не шире терминала (рамки не едут).
func TestTallSymbols_ScreenLinesMatchTerminalWidth(t *testing.T) {
	syms := []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"}
	for _, f := range []banner.Font{banner.FontMini, banner.FontSmall, banner.FontStandard, banner.FontBanner3Compact, banner.FontBanner3Mini} {
		for _, w := range []int{114, 125, 140, 180, 260} {
			m := tallModel(t, f, syms, w, 44)
			for i, line := range strings.Split(m.render(), "\n") {
				if got := lipgloss.Width(strings.TrimRight(line, " ")); got > w {
					t.Fatalf("%s w=%d строка %d шире терминала: %d", f, w, i, got)
				}
			}
		}
	}
}

// Тикер — котируемая монета, а не весь символ; крупно, на широкой панели.
func TestTallSymbols_ShowsTickerNotFullSymbol(t *testing.T) {
	m := tallModel(t, banner.FontSmall, []string{"BTC_USDT"}, 180, 40)
	s := plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
	fig := plain(banner.Render("BTC", banner.Options{Font: banner.FontSmall}))
	for _, line := range strings.Split(fig, "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("в панели нет строки FIGlet-тикера %q:\n%s", line, s)
		}
	}
	if strings.Contains(s, "USDT") {
		t.Errorf("в крупном режиме не должно быть «USDT»:\n%s", s)
	}
}

// На узкой панели — прежний вид: имя символа текстом.
func TestTallSymbols_NarrowFallsBackToPlainNames(t *testing.T) {
	m := tallModel(t, banner.FontStandard, []string{"BTC_USDT"}, 100, 30)
	s := plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
	if !strings.Contains(s, "BTC_USDT") {
		t.Errorf("на узкой панели ожидалось имя символа текстом:\n%s", s)
	}
}

// Курсор двигается и не выходит за границы; выбранная строка видна и при
// прокрутке (символов больше, чем помещается).
func TestTallSymbols_CursorAndScroll(t *testing.T) {
	syms := []string{"AAA_USDT", "BBB_USDT", "CCC_USDT", "DDD_USDT", "EEE_USDT", "FFF_USDT", "GGG_USDT", "HHH_USDT"}
	m := tallModel(t, banner.FontStandard, syms, 180, 30)
	press := func(k string) {
		var ok bool
		m, ok = m.moveCursor(k)
		if !ok {
			t.Fatalf("клавиша %q не обработана", k)
		}
	}
	press("up")
	if m.cursor != 0 {
		t.Errorf("курсор вышел за верх: %d", m.cursor)
	}
	press("end")
	if m.cursor != len(syms)-1 {
		t.Errorf("end: cursor=%d", m.cursor)
	}
	press("down")
	if m.cursor != len(syms)-1 {
		t.Errorf("курсор вышел за низ: %d", m.cursor)
	}
	s := plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
	fig := plain(banner.Render("HHH", banner.Options{Font: banner.FontStandard}))
	for _, line := range strings.Split(fig, "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("выбранная (последняя) строка не видна после прокрутки:\n%s", s)
		}
	}
	if _, ok := m.moveCursor("x"); ok {
		t.Error("посторонняя клавиша не должна обрабатываться")
	}
}

// Без флага ничего не меняется: рисуется обычная таблица.
func TestTallSymbols_OffByDefault(t *testing.T) {
	m := tallModel(t, "", []string{"BTC_USDT"}, 180, 40)
	if !strings.Contains(plain(m.render()), "BTC_USDT") {
		t.Error("без -ticker-font должна быть обычная таблица с полным именем символа")
	}
}

// Каждый шрифт кита (banner.Fonts) рисует тикер в таблице без выхода за
// размеры панели: проверка ширины и высоты панели на нескольких ширинах
// терминала. Новый шрифт в ките попадёт сюда автоматически.
func TestTickerFonts_EveryKitFontFitsPanel(t *testing.T) {
	for _, f := range banner.Fonts() {
		for _, w := range []int{114, 140, 180, 260} {
			m := tallModel(t, f, []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"}, w, 44)
			cw, ch := m.contentWidth()-2, m.bodyHeight()-2
			s := m.renderSymbolsTall(cw, ch)
			if got := lipgloss.Width(s); got != cw {
				t.Fatalf("%s w=%d: ширина панели %d, want %d", f, w, got, cw)
			}
			if got := lipgloss.Height(s); got != ch {
				t.Fatalf("%s w=%d: высота панели %d, want %d", f, w, got, ch)
			}
		}
	}
}

// banner3-mini в таблице: строка из трёх строк знака плюс зазор, тикер
// узнаваем (в панели есть первая строка рисунка из кита).
func TestTallSymbols_Banner3MiniRowHeight(t *testing.T) {
	m := tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT"}, 180, 40)
	s := plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
	fig := strings.Split(strings.TrimRight(plain(banner.Render("BTC", banner.Options{Font: banner.FontBanner3Mini})), "\n"), "\n")
	if len(fig) != 3 {
		t.Fatalf("banner3-mini должен давать 3 строки, а даёт %d", len(fig))
	}
	for _, line := range fig {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("в панели нет строки тикера %q:\n%s", line, s)
		}
	}
}

// Заголовки и значения остальных колонок не усекаются, пока включён крупный
// режим: он включается только когда под них хватает места.
func TestTallSymbols_HeadersNotTruncated(t *testing.T) {
	for _, f := range []banner.Font{banner.FontBanner3, banner.FontBanner3Compact, banner.FontBanner3Mini, banner.FontStandard, banner.FontSmall} {
		for _, w := range []int{140, 160, 180, 220} {
			m := tallModel(t, f, []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"}, w, 44)
			s := plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
			for _, h := range tallHeaders {
				if !strings.Contains(s, h) {
					t.Errorf("%s w=%d: заголовок %q усечён или пропал:\n%s", f, w, h, s)
				}
			}
		}
	}
}
