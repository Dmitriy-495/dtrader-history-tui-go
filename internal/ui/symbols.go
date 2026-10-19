// Крупные тикеры (флаг -ticker-font, по умолчанию banner3-mini): левая
// панель рисуется не bubbles/table, а своим рендером, потому что таблица
// строго однострочная, а FIGlet-имя монеты занимает 3–7 строк. Строка
// символа без заголовков столбцов состоит из трёх частей:
//
//   - тикер — котируемая монета (BTC из BTC_USDT) шрифтом из dtui-kit/banner;
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

	// Самое широкое значение возраста — по нему считается ширина колонки,
	// чтобы колонки не «ездили» при смене цифр.
	ageWidest = "88.8s"
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
// Колонки: тикер · возраст · кружок. Если ширины не хватает, крупный вид
// заменяется текстовым (plainSymbolLines).
func (m Model) renderSymbolsTall(width, height int) string {
	now := nowFn()
	render := func(text string) string {
		return strings.TrimRight(banner.Render(text, banner.Options{Font: m.tickerFont}), "\n")
	}

	// Рисунки тикеров; ширина колонок считается по самым широким значениям.
	figs := make([]string, len(m.symbols))
	figW, rowH := 0, dotH
	for i, sym := range m.symbols {
		figs[i] = render(tickerOf(sym))
		figW = max(figW, lipgloss.Width(figs[i]))
		rowH = max(rowH, lipgloss.Height(figs[i]))
	}
	widest := render(ageWidest)
	rowH = max(rowH, lipgloss.Height(widest))

	tickCol, ageCol, dotCol := figW+2, lipgloss.Width(widest)+2, dotW+2
	need := tickCol + ageCol + dotCol + 2*tallGapMin

	var lines []string
	if width < need {
		lines = m.plainSymbolLines(now)
	} else {
		// Свободное место делится на зазоры между колонками (до tallGapMax).
		gap := tallGapMin + min(tallGapMax-tallGapMin, (width-need)/2)
		widths := []int{tickCol + gap, ageCol + gap, dotCol}

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

			tickStyle, ageStyle := theme.DataStyle, lipgloss.NewStyle()
			if i == m.cursor {
				tickStyle, ageStyle = tallSelectedStyle, tallSelectedStyle
			}
			switch kind {
			case dotNone:
				ageStyle = theme.MutedStyle
			case dotLost:
				ageStyle = theme.SOSStyle
			}

			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
				cell(tickStyle.Render(figs[i]), widths[0], rowH),
				cell(ageStyle.Render(render(ageLabel(v))), widths[1], rowH),
				cell(renderDot(kind, now), widths[2], rowH),
			))
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
// тикер и возраст обычным текстом, затем кружок «●» (или «○», если данных
// нет) тем же цветом. Заголовков нет.
func (m Model) plainSymbolLines(now time.Time) []string {
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
		lines = append(lines, " "+tick.Render(fmt.Sprintf("%-6s", tickerOf(sym)))+
			fmt.Sprintf("%-7s", ageLabel(v))+
			lipgloss.NewStyle().Foreground(dotColor(kind, now)).Render(glyph))
	}
	return lines
}
