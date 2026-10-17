// Крупные тикеры (флаг -ticker-font, по умолчанию banner3-mini): левая
// панель рисуется не bubbles/table, а своим рендером, потому что таблица
// строго однострочная, а FIGlet-имя монеты занимает 3–7 строк. Строка
// символа без заголовков столбцов состоит из трёх частей:
//
//   - тикер — котируемая монета (BTC из BTC_USDT) шрифтом из dtui-kit/banner;
//   - цена — середина лучшего бида и аска из последнего снапшота сборщика
//     (поля best_bid/best_ask статуса), тем же шрифтом; колонка появляется,
//     только когда сборщик цены присылает, на узкой панели пропадает первой;
//   - возраст последней записи тем же шрифтом (0.4s, 12s, 5m, 2h; нет данных —
//     «--»);
//   - кружок состояния: зелёный пульсирующий — данные идут; ярко-красный —
//     запись застряла или оборвана связь со сборщиком (в том числе до первых
//     данных); серый пустой — идёт первое подключение, данных ещё не было.
//
// Выбранная строка (↑↓, j/k, Home/End) подсвечивается розовым, как в
// bubbles/table. Без крупных тикеров (-ticker-font off) этот код не
// используется и рисуется прежняя таблица. Эксперимент: когда режим
// утвердят окончательно, он заменит bubbles/table.
package ui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

const (
	tallRowGap = 1 // пустых строк между строками символов
	tallTopPad = 1 // пустых строк над первой строкой (как у правой панели)
	tallGapMin = 4 // минимальный зазор между колонками
	tallGapMax = 12

	dotW = 6 // ширина кружка в ячейках
	dotH = 3 // высота кружка в строках

	// Пульс зелёного кружка: период и доля яркости в самой тёмной точке.
	pulsePeriod = 2 * time.Second
	pulseFloor  = 0.25

	// Самые широкие значения возраста и цены — по ним считается ширина
	// колонок, чтобы колонки не «ездили» при смене цифр.
	ageWidest   = "88.8s"
	priceWidest = "99999.99"
)

// nowFn — источник времени для пульса; подменяется в тестах.
var nowFn = time.Now

var tallSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true) // как у bubbles/table

// Кружок 6×3 из блочных знаков: залитый и контурный (нет данных).
var (
	dotFilled = []string{" ▗▄▄▖ ", "▐████▌", " ▝▀▀▘ "}
	dotHollow = []string{" ▗▄▄▖ ", "▐▌  ▐▌", " ▝▀▀▘ "}
)

// dotKind — вид кружка состояния символа.
type dotKind int

const (
	dotNone dotKind = iota // идёт первое подключение, данных по символу ещё не было
	dotOK                  // данные идут
	dotLost                // запись застряла или оборвана связь со сборщиком
)

// symbolDot определяет вид кружка. linkLost — связь со сборщиком потеряна
// (модель хранит последнюю ошибку клиента до первого успешного статуса).
// Потеря связи — ярко-красный даже там, где данных ещё не было; запись
// застряла — тоже ярко-красный; данные идут — зелёный; идёт первое
// подключение (данных нет, ошибки нет) — пустой серый.
func symbolDot(v symView, linkLost bool) dotKind {
	switch {
	case linkLost:
		return dotLost
	case !v.hasData:
		return dotNone
	case v.stale:
		return dotLost
	default:
		return dotOK
	}
}

// pulseColor — цвет зелёного кружка в момент t: яркость плавно меняется от
// pulseFloor до полной по косинусу с периодом pulsePeriod. При тике 500 мс
// это четыре ступени за период.
func pulseColor(t time.Time) color.Color {
	ms := pulsePeriod.Milliseconds()
	phase := float64(t.UnixMilli()%ms) / float64(ms)
	k := 0.5 - 0.5*math.Cos(2*math.Pi*phase) // 0…1
	f := pulseFloor + (1-pulseFloor)*k
	r, g, b, _ := theme.ColorOK.RGBA()
	return color.RGBA{R: uint8(float64(r>>8) * f), G: uint8(float64(g>>8) * f), B: uint8(float64(b>>8) * f), A: 255}
}

// dotColor — цвет кружка вида kind в момент now.
func dotColor(kind dotKind, now time.Time) color.Color {
	switch kind {
	case dotOK:
		return pulseColor(now)
	case dotNone:
		return theme.ColorMuted
	default:
		return theme.ColorSOS
	}
}

// renderDot рисует кружок вида kind (три строки) в момент now.
func renderDot(kind dotKind, now time.Time) string {
	art := dotFilled
	if kind == dotNone {
		art = dotHollow
	}
	return lipgloss.NewStyle().Foreground(dotColor(kind, now)).Render(strings.Join(art, "\n"))
}

// formatAge — возраст последней записи для крупного вывода (только знаки,
// которые есть в banner3: цифры, точка, s/m/h): 0.4s, 12s, 5m, 2h; неизвестен — «--».
func formatAge(age float64) string {
	switch {
	case age < 0:
		return "--"
	case age < 10:
		return fmt.Sprintf("%.1fs", math.Floor(age*10)/10)
	case age < 60:
		return fmt.Sprintf("%ds", int(age))
	case age < 3600:
		return fmt.Sprintf("%dm", int(age/60))
	default:
		return fmt.Sprintf("%dh", int(age/3600))
	}
}

// midPrice — середина лучшего бида и аска, записанная с тем же числом знаков
// после точки, что у исходных цен (плюс один, если нужен для середины, без
// лишних нулей в конце). Пустая строка — цены нет или она не разобрана.
func midPrice(bid, ask string) string {
	b, errB := strconv.ParseFloat(bid, 64)
	a, errA := strconv.ParseFloat(ask, 64)
	if errB != nil || errA != nil || b <= 0 || a <= 0 {
		return ""
	}
	dec := max(decimals(bid), decimals(ask))
	s := strconv.FormatFloat((a+b)/2, 'f', dec+1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		for len(s)-i-1 > dec && s[len(s)-1] == '0' {
			s = s[:len(s)-1]
		}
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

// decimals — число знаков после точки в записи числа.
func decimals(s string) int {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// anyPrice — сборщик прислал цену хотя бы по одному символу (иначе колонку
// цены не рисуем: она показывала бы одни прочерки).
func (m Model) anyPrice() bool {
	for _, sym := range m.symbols {
		if m.symView(sym).price != "" {
			return true
		}
	}
	return false
}

// tickerOf — котируемая монета: всё до первого «_» (BTC_USDT → BTC).
func tickerOf(symbol string) string {
	if i := strings.IndexByte(symbol, '_'); i > 0 {
		return symbol[:i]
	}
	return symbol
}

// WithTickerFont включает режим крупных тикеров шрифтом font ("" — выключен).
func (m Model) WithTickerFont(font banner.Font) Model {
	m.tickerFont = font
	return m
}

// moveCursor двигает выбранную строку в режиме крупных тикеров; ok — клавиша
// обработана.
func (m Model) moveCursor(key string) (Model, bool) {
	last := len(m.symbols) - 1
	switch key {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = last
	default:
		return m, false
	}
	m.cursor = max(0, min(m.cursor, last))
	return m, true
}

// cell рисует ячейку шириной w и высотой h: содержимое усекается (не
// переносится), по бокам по одному пробелу отступа, по вертикали по центру.
func cell(s string, w, h int) string {
	inner := lipgloss.NewStyle().MaxWidth(max(w-2, 1)).Render(s)
	return lipgloss.NewStyle().
		Padding(0, 1).Width(w).Height(h).
		AlignVertical(lipgloss.Center).
		Render(inner)
}

// renderSymbolsTall строит содержимое левой панели ровно width × height.
// Колонки: тикер · [цена] · возраст · кружок. Если ширины не хватает, сначала
// пропадает цена, затем крупный вид заменяется текстовым (plainSymbolLines).
func (m Model) renderSymbolsTall(width, height int) string {
	now := nowFn()
	render := func(text string) string {
		return strings.TrimRight(banner.Render(text, banner.Options{Font: m.tickerFont}), "\n")
	}
	withPrice := m.anyPrice()

	// Рисунки тикеров и цен; ширина колонок считается по самым широким значениям.
	figs := make([]string, len(m.symbols))
	prices := make([]string, len(m.symbols))
	figW, rowH := 0, dotH
	priceW := lipgloss.Width(render(priceWidest))
	for i, sym := range m.symbols {
		figs[i] = render(tickerOf(sym))
		figW = max(figW, lipgloss.Width(figs[i]))
		rowH = max(rowH, lipgloss.Height(figs[i]))
		if withPrice {
			text := m.symView(sym).price
			if text == "" {
				text = "--"
			}
			prices[i] = render(text)
			priceW = max(priceW, lipgloss.Width(prices[i]))
		}
	}
	widest := render(ageWidest)
	rowH = max(rowH, lipgloss.Height(widest))

	tickCol, ageCol, dotCol, priceCol := figW+2, lipgloss.Width(widest)+2, dotW+2, priceW+2
	cols := 3 // тикер, возраст, кружок
	need := tickCol + ageCol + dotCol + (cols-1)*tallGapMin
	showPrice := withPrice && width >= need+priceCol+tallGapMin
	if showPrice {
		cols, need = cols+1, need+priceCol+tallGapMin
	}

	var lines []string
	if width < need {
		lines = m.plainSymbolLines(now, withPrice)
	} else {
		// Свободное место делится на зазоры между колонками (до tallGapMax).
		gap := tallGapMin + min(tallGapMax-tallGapMin, (width-need)/(cols-1))
		widths := []int{tickCol + gap}
		if showPrice {
			widths = append(widths, priceCol+gap)
		}
		widths = append(widths, ageCol+gap, dotCol)

		visible := max(1, (height-tallTopPad+tallRowGap)/(rowH+tallRowGap))
		offset := 0
		if m.cursor >= visible {
			offset = m.cursor - visible + 1
		}
		end := min(offset+visible, len(m.symbols))

		blank := strings.Repeat(" ", width)
		for i := 0; i < tallTopPad; i++ {
			lines = append(lines, blank)
		}
		for i := offset; i < end; i++ {
			if i > offset {
				lines = append(lines, blank)
			}
			v := m.symView(m.symbols[i])
			kind := symbolDot(v, m.lastErr != nil)

			tickStyle, ageStyle, priceStyle := theme.DataStyle, lipgloss.NewStyle(), lipgloss.NewStyle().Bold(true)
			if i == m.cursor {
				tickStyle, ageStyle, priceStyle = tallSelectedStyle, tallSelectedStyle, tallSelectedStyle
			}
			switch kind {
			case dotNone:
				ageStyle, priceStyle = theme.MutedStyle, theme.MutedStyle
			case dotLost:
				// Цена застывшая — не доверяем ей, приглушаем; красным горит возраст.
				ageStyle, priceStyle = theme.SOSStyle, theme.MutedStyle
			}

			cells := []string{cell(tickStyle.Render(figs[i]), widths[0], rowH)}
			next := 1
			if showPrice {
				cells = append(cells, cell(priceStyle.Render(prices[i]), widths[next], rowH))
				next++
			}
			cells = append(cells,
				cell(ageStyle.Render(render(ageLabel(v))), widths[next], rowH),
				cell(renderDot(kind, now), widths[next+1], rowH),
			)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		}
	}

	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Render(strings.Join(lines, "\n"))
}

// ageLabel — текст возраста для символа: «--», если данных нет.
func ageLabel(v symView) string {
	if !v.hasData {
		return "--"
	}
	return formatAge(v.age)
}

// plainSymbolLines — запасной вид для узкой панели: по строке на символ,
// тикер, цена (если сборщик её присылает) и возраст обычным текстом, затем
// кружок «●» (или «○», если данных нет) тем же цветом. Заголовков нет.
func (m Model) plainSymbolLines(now time.Time, withPrice bool) []string {
	lines := make([]string, 0, len(m.symbols)+tallTopPad)
	for i := 0; i < tallTopPad; i++ {
		lines = append(lines, "")
	}
	for i, sym := range m.symbols {
		v := m.symView(sym)
		kind := symbolDot(v, m.lastErr != nil)
		glyph := "●"
		if kind == dotNone {
			glyph = "○"
		}
		tick := theme.DataStyle
		if i == m.cursor {
			tick = tallSelectedStyle
		}
		price := ""
		if withPrice {
			text := v.price
			if text == "" {
				text = "--"
			}
			price = fmt.Sprintf("%-10s", text)
		}
		lines = append(lines, " "+tick.Render(fmt.Sprintf("%-6s", tickerOf(sym)))+price+
			fmt.Sprintf("%-7s", ageLabel(v))+
			lipgloss.NewStyle().Foreground(dotColor(kind, now)).Render(glyph))
	}
	return lines
}
