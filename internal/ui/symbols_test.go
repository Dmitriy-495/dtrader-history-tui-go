package ui

import (
	"image/color"
	"strings"
	"testing"
	"time"

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

// tallModel — модель в режиме крупных тикеров font размером w×h, по всем
// символам пришёл статус с возрастом ageSecs.
func tallModelAge(t *testing.T, font banner.Font, symbols []string, w, h int, ageSecs float64) Model {
	t.Helper()
	m := newTestModel("h", 1, "t", symbols).WithTickerFont(font)
	u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = u.(Model)
	st := statusclient.Status{Type: "status", UptimeSeconds: 10, Symbols: map[string]statusclient.SymbolStatus{}}
	for _, s := range symbols {
		st.Symbols[s] = statusclient.SymbolStatus{SnapshotsSinceStart: 123456, LastSnapshotAgeSecs: ageSecs}
	}
	u, _ = m.Update(statusMsg(st))
	return u.(Model)
}

func tallModel(t *testing.T, font banner.Font, symbols []string, w, h int) Model {
	t.Helper()
	return tallModelAge(t, font, symbols, w, h, 0.4)
}

// panel — содержимое левой панели без цветов.
func panel(m Model) string {
	return plain(m.renderSymbolsTall(m.contentWidth()-2, m.bodyHeight()-2))
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
	s := panel(m)
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

// Заголовков столбцов и колонки «Снапшотов» больше нет.
func TestTallSymbols_NoHeadersNoSnapshotsColumn(t *testing.T) {
	m := tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT"}, 180, 40)
	s := panel(m)
	for _, banned := range []string{"Символ", "Снапшотов", "Последняя запись", "Статус", "123456"} {
		if strings.Contains(s, banned) {
			t.Errorf("в панели не должно быть %q:\n%s", banned, s)
		}
	}
}

// Возраст рисуется тем же шрифтом, что и тикер.
func TestTallSymbols_AgeInTickerFont(t *testing.T) {
	m := tallModelAge(t, banner.FontBanner3Mini, []string{"BTC_USDT"}, 180, 40, 3.2)
	s := panel(m)
	fig := plain(banner.Render("3.2s", banner.Options{Font: banner.FontBanner3Mini}))
	for _, line := range strings.Split(strings.TrimRight(fig, "\n"), "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("в панели нет строки возраста %q:\n%s", line, s)
		}
	}
}

// На узкой панели — запасной вид: ярлык «BTC», возраст и кружок текстом.
func TestTallSymbols_NarrowFallsBackToPlainText(t *testing.T) {
	m := tallModel(t, banner.FontStandard, []string{"BTC_USDT"}, 100, 30)
	s := panel(m)
	for _, want := range []string{"BTC", "0.4s", "●"} {
		if !strings.Contains(s, want) {
			t.Errorf("на узкой панели ожидалось %q:\n%s", want, s)
		}
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
	s := panel(m)
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

// Модель сама по себе без крупных тикеров: рисуется обычная таблица
// (включает их main.go, значением флага по умолчанию).
func TestTallSymbols_ModelPlainTableByDefault(t *testing.T) {
	m := tallModel(t, "", []string{"BTC_USDT"}, 180, 40)
	if !strings.Contains(plain(m.render()), "BTC_USDT") {
		t.Error("модель без шрифта должна рисовать обычную таблицу с полным именем символа")
	}
}

// Каждый шрифт кита (banner.Fonts) рисует тикер в таблице без выхода за
// размеры панели. Новый шрифт в ките попадёт сюда автоматически.
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

func TestFormatAge(t *testing.T) {
	for _, c := range []struct {
		age  float64
		want string
	}{
		{-1, "--"}, {0, "0.0s"}, {0.44, "0.4s"}, {0.96, "0.9s"}, {9.99, "9.9s"},
		{10, "10s"}, {12.7, "12s"}, {59.9, "59s"}, {60, "1m"}, {300, "5m"}, {3599, "59m"}, {3600, "1h"}, {7300, "2h"},
	} {
		if got := formatAge(c.age); got != c.want {
			t.Errorf("formatAge(%v) = %q, want %q", c.age, got, c.want)
		}
	}
	// Все знаки возраста умеет рисовать banner3-mini (3 строки).
	for _, s := range []string{"0.4s", "88.8s", "12s", "5m", "2h", "--"} {
		if h := lipgloss.Height(strings.TrimRight(banner.Render(s, banner.Options{Font: banner.FontBanner3Mini}), "\n")); h < 2 || h > 3 {
			t.Errorf("%q: высота %d, ожидалось 2–3", s, h)
		}
	}
}

// Вид кружка: зелёный при живых данных, ярко-красный при застрявшей записи
// или потере связи (в том числе до первых данных), серый пустой — только
// пока идёт первое подключение. lost — «связь со сборщиком потеряна».
func TestSymbolDot(t *testing.T) {
	fresh := symView{hasData: true, age: 0.4}
	stale := symView{hasData: true, age: 9, stale: true}
	for name, c := range map[string]struct {
		v    symView
		lost bool
		want dotKind
	}{
		"данные идут":                {fresh, false, dotOK},
		"запись застряла":            {stale, false, dotLost},
		"связь потеряна":             {fresh, true, dotLost},
		"застряла и связь потеряна":  {stale, true, dotLost},
		"первое подключение":         {symView{}, false, dotNone},
		"данных нет, связь потеряна": {symView{}, true, dotLost},
	} {
		if got := symbolDot(c.v, c.lost); got != c.want {
			t.Errorf("%s: symbolDot = %d, want %d", name, got, c.want)
		}
	}
}

// Пульс: период 2 секунды, ярче и темнее в разных фазах, возврат через период.
func TestPulseColor(t *testing.T) {
	base := time.UnixMilli(1_000_000_000_000 - 1_000_000_000_000%2000) // граница периода
	lum := func(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return r + g + b }
	dim, bright := lum(pulseColor(base)), lum(pulseColor(base.Add(time.Second)))
	if dim >= bright {
		t.Errorf("в начале периода кружок должен быть темнее, чем через 1 с: dim=%d bright=%d", dim, bright)
	}
	if lum(pulseColor(base.Add(500*time.Millisecond))) <= dim || lum(pulseColor(base.Add(500*time.Millisecond))) >= bright {
		t.Error("через 0,5 с яркость должна быть между минимумом и максимумом")
	}
	if pulseColor(base) != pulseColor(base.Add(2*time.Second)) {
		t.Error("цвет должен повторяться через период (2 с)")
	}
	r, g, b, _ := pulseColor(base).RGBA()
	if g <= r || g <= b {
		t.Errorf("кружок должен быть зелёным даже в самой тёмной точке: r=%d g=%d b=%d", r, g, b)
	}
}

// Кружок на экране: залитый (данные идут / обрыв) и пустой (данных нет).
func TestTallSymbols_DotShapes(t *testing.T) {
	filled, hollow := dotFilled[1], dotHollow[1]
	m := tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT"}, 180, 40)
	// SOL — без данных: убираем символ из статуса.
	delete(m.lastStatus.Symbols, "SOL_USDT")
	s := panel(m)
	if !strings.Contains(s, filled) {
		t.Errorf("нет залитого кружка:\n%s", s)
	}
	if !strings.Contains(s, hollow) {
		t.Errorf("нет пустого кружка для символа без данных:\n%s", s)
	}
}

// Каждый шрифт кита рисует тикер банера: banner3-mini ровно три строки.
func TestTallSymbols_Banner3MiniRowHeight(t *testing.T) {
	m := tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT"}, 180, 40)
	s := panel(m)
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

// Ошибка подключения до первых данных: все кружки ярко-красные и залитые,
// пустого серого контура нет (подвал в этот момент тоже красный).
func TestTallSymbols_LinkLostBeforeAnyDataIsRed(t *testing.T) {
	m := newTestModel("h", 1, "t", []string{"BTC_USDT", "SOL_USDT"}).WithTickerFont(banner.FontBanner3Mini)
	u, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 40})
	m = u.(Model)

	s := panel(m) // идёт первое подключение: серые пустые кружки
	if !strings.Contains(s, dotHollow[1]) || strings.Contains(s, dotFilled[1]) {
		t.Fatalf("до ошибки ожидались серые пустые кружки:\n%s", s)
	}

	u, _ = m.Update(clientErrMsg{err: errTest})
	m = u.(Model)
	s = panel(m)
	if !strings.Contains(s, dotFilled[1]) || strings.Contains(s, dotHollow[1]) {
		t.Fatalf("после потери связи ожидались залитые красные кружки:\n%s", s)
	}
	for _, v := range []string{"BTC_USDT", "SOL_USDT"} {
		if got := symbolDot(m.symView(v), m.lastErr != nil); got != dotLost {
			t.Errorf("%s: вид кружка %d, want dotLost", v, got)
		}
	}

	// Пришёл статус — ошибка снимается, кружки снова зелёные.
	u, _ = m.Update(statusMsg(statusclient.Status{Type: "status", Symbols: map[string]statusclient.SymbolStatus{
		"BTC_USDT": {LastSnapshotAgeSecs: 0.3}, "SOL_USDT": {LastSnapshotAgeSecs: 0.3}}}))
	m = u.(Model)
	if got := symbolDot(m.symView("BTC_USDT"), m.lastErr != nil); got != dotOK {
		t.Errorf("после статуса вид кружка %d, want dotOK", got)
	}
}

func TestMidPrice(t *testing.T) {
	for _, c := range []struct{ bid, ask, want string }{
		{"97250.1", "97250.2", "97250.15"},
		{"97250.1", "97250.3", "97250.2"},
		{"0.1234", "0.1236", "0.1235"},
		{"24", "25", "24.5"},
		{"100", "100", "100"},
		{"145.32", "145.34", "145.33"},
		{"", "1", ""},
		{"x", "1", ""},
		{"0", "1", ""},
		{"-5", "1", ""},
	} {
		if got := midPrice(c.bid, c.ask); got != c.want {
			t.Errorf("midPrice(%q,%q) = %q, want %q", c.bid, c.ask, got, c.want)
		}
	}
}

// withPrices кладёт в статус лучшие цены для всех символов модели.
func withPrices(m Model, bid, ask string) Model {
	for sym, ss := range m.lastStatus.Symbols {
		ss.BestBid, ss.BestAsk = statusclient.Price(bid), statusclient.Price(ask)
		m.lastStatus.Symbols[sym] = ss
	}
	return m
}

// Колонки цены нет, пока сборщик цены не присылает (текущий сборщик): панель
// выглядит как до появления цены.
func TestTallSymbols_NoPriceColumnWithoutPrices(t *testing.T) {
	m := tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT"}, 180, 40)
	if m.anyPrice() {
		t.Fatal("без best_bid/best_ask цены быть не должно")
	}
	s := panel(m)
	price := plain(banner.Render("97250.15", banner.Options{Font: banner.FontBanner3Mini}))
	if first := strings.TrimRight(strings.Split(price, "\n")[0], " "); strings.Contains(s, first) {
		t.Errorf("колонки цены быть не должно:\n%s", s)
	}
}

// Цена — середина бида и аска, тем же шрифтом, что тикер.
func TestTallSymbols_PriceIsMidOfBestBidAsk(t *testing.T) {
	m := withPrices(tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT"}, 180, 40), "97250.1", "97250.2")
	if got := m.symView("BTC_USDT").price; got != "97250.15" {
		t.Fatalf("price = %q, want 97250.15", got)
	}
	s := panel(m)
	fig := plain(banner.Render("97250.15", banner.Options{Font: banner.FontBanner3Mini}))
	for _, line := range strings.Split(strings.TrimRight(fig, "\n"), "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("в панели нет строки цены %q:\n%s", line, s)
		}
	}
}

// Если цена не помещается по ширине, пропадает именно она, а крупный вид
// остаётся; когда не помещается и он — запасной текстовый вид с ценой. Ширина
// панели всегда точная, цена, однажды появившись, при росте ширины не пропадает.
func TestTallSymbols_PriceDropsFirstOnNarrowPanel(t *testing.T) {
	m := withPrices(tallModel(t, banner.FontBanner3Mini, []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"}, 180, 40), "97250.1", "97250.2")
	priceFirst := strings.TrimRight(strings.Split(plain(banner.Render("97250.15", banner.Options{Font: banner.FontBanner3Mini})), "\n")[0], " ")
	tickFirst := strings.TrimRight(strings.Split(plain(banner.Render("BTC", banner.Options{Font: banner.FontBanner3Mini})), "\n")[0], " ")

	seenPrice := false
	for w := 40; w <= 200; w++ {
		raw := m.renderSymbolsTall(w, 40)
		if lipgloss.Width(raw) != w || lipgloss.Height(raw) != 40 {
			t.Fatalf("w=%d: размер %dx%d, want %dx40", w, lipgloss.Width(raw), lipgloss.Height(raw), w)
		}
		s := plain(raw)
		hasPrice := strings.Contains(s, priceFirst)
		if seenPrice && !hasPrice {
			t.Fatalf("w=%d: цена пропала при росте ширины", w)
		}
		seenPrice = seenPrice || hasPrice
		if hasPrice && !strings.Contains(s, tickFirst) {
			t.Fatalf("w=%d: цена есть, а крупного тикера нет", w)
		}
	}
	if !seenPrice {
		t.Error("на ширине 200 цена должна помещаться")
	}
	if narrow := plain(m.renderSymbolsTall(40, 40)); !strings.Contains(narrow, "97250.15") || !strings.Contains(narrow, "BTC") {
		t.Errorf("в запасном виде ожидались тикер и цена текстом:\n%s", narrow)
	}
}

// Застывшая цена (запись застряла или связь потеряна) приглушается, но не
// исчезает: цифры остаются на экране.
func TestTallSymbols_StalePriceStaysVisible(t *testing.T) {
	m := withPrices(tallModelAge(t, banner.FontBanner3Mini, []string{"BTC_USDT"}, 180, 40, 30), "97250.1", "97250.2")
	s := panel(m)
	fig := plain(banner.Render("97250.15", banner.Options{Font: banner.FontBanner3Mini}))
	for _, line := range strings.Split(strings.TrimRight(fig, "\n"), "\n") {
		if strings.TrimSpace(line) != "" && !strings.Contains(s, strings.TrimRight(line, " ")) {
			t.Fatalf("застывшая цена пропала с экрана:\n%s", s)
		}
	}
}
