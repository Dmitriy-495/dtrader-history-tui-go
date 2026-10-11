// Режим «крупные тикеры» (флаг -ticker-font): левая панель рисуется не
// bubbles/table, а своим рендером, потому что таблица строго однострочная,
// а FIGlet-имя монеты занимает 3–7 строк. В первой колонке — тикер
// (котируемая монета: BTC из BTC_USDT) шрифтом из dtui-kit/banner
// (рекомендуемый — banner3-mini, 3 строки), остальные колонки по центру
// строки по вертикали. Данные те же, что у таблицы (buildRows). Без флага
// этот код не используется. Эксперимент: когда режим утвердят окончательно,
// он заменит bubbles/table.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

const (
	tallRowGap = 1 // пустых строк между строками
	// Минимальные ширины трёх остальных колонок (по заголовку и содержимому,
	// с отступами): «Снапшотов», «Последняя запись», «ЗАСТРЯЛ?». Меньше —
	// заголовки усекаются.
	tallMinSnap, tallMinAge, tallMinStatus = 11, 18, 10
	tallMinRest                            = tallMinSnap + tallMinAge + tallMinStatus
)

var (
	tallHeaders       = []string{"Символ", "Снапшотов", "Последняя запись", "Статус"}
	tallHeaderStyle   = lipgloss.NewStyle().Bold(true)
	tallSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true) // как у bubbles/table
)

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

// cell рисует ячейку шириной w и высотой h: текст усекается (не
// переносится), по бокам по одному пробелу отступа, по вертикали по центру.
func cell(s string, w, h int, st lipgloss.Style) string {
	inner := lipgloss.NewStyle().MaxWidth(max(w-2, 1)).Render(st.Render(s))
	return lipgloss.NewStyle().
		Padding(0, 1).Width(w).Height(h).
		AlignVertical(lipgloss.Center).
		Render(inner)
}

// renderSymbolsTall строит содержимое левой панели ровно width × height.
func (m Model) renderSymbolsTall(width, height int) string {
	rows := m.buildRows()
	figs := make([]string, len(rows))
	figW, rowH := 0, 1
	for i, sym := range m.symbols {
		figs[i] = strings.TrimRight(banner.Render(tickerOf(sym), banner.Options{Font: m.tickerFont}), "\n")
		figW = max(figW, lipgloss.Width(figs[i]))
		rowH = max(rowH, lipgloss.Height(figs[i]))
	}

	// Крупные тикеры, если под остальные колонки остаётся место; иначе
	// на узкой панели — прежний вид (имя символа текстом, строка в одну линию).
	tickCol := figW + 2
	big := width-tickCol >= tallMinRest
	var widths []int
	if big {
		// Сначала каждой колонке минимум, остаток делится 3:3:2.
		extra := width - tickCol - tallMinRest
		w1 := tallMinSnap + extra*3/8
		w2 := tallMinAge + extra*3/8
		w3 := width - tickCol - w1 - w2
		widths = []int{tickCol, w1, w2, w3}
	} else {
		widths = proportionalWidths(width, []int{2, 3, 3, 2})
		rowH = 1
	}
	gap := tallRowGap
	if !big {
		gap = 0
	}

	var lines []string
	hdr := make([]string, 4)
	for c, title := range tallHeaders {
		hdr[c] = cell(title, widths[c], 1, tallHeaderStyle)
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, hdr...))

	// Сколько строк помещается; окно прокручивается так, чтобы выбранная
	// строка всегда была видна (без состояния: смещение выводится из курсора).
	avail := height - 1 - gap
	visible := max(1, (avail+gap)/(rowH+gap))
	offset := 0
	if m.cursor >= visible {
		offset = m.cursor - visible + 1
	}
	end := min(offset+visible, len(rows))

	for i := offset; i < end; i++ {
		if gap > 0 {
			lines = append(lines, strings.Repeat(" ", width))
		}
		st := lipgloss.NewStyle()
		fig := theme.DataStyle.Render(figs[i])
		if i == m.cursor {
			st = tallSelectedStyle
			fig = tallSelectedStyle.Render(figs[i])
		}
		cells := make([]string, 4)
		if big {
			cells[0] = lipgloss.NewStyle().Padding(0, 1).Width(widths[0]).Height(rowH).AlignVertical(lipgloss.Center).Render(fig)
		} else {
			cells[0] = cell(m.symbols[i], widths[0], 1, st)
		}
		cells[1] = cell(rows[i][1], widths[1], rowH, st)
		cells[2] = cell(rows[i][2], widths[2], rowH, st)
		cells[3] = cell(rows[i][3], widths[3], rowH, lipgloss.NewStyle()) // статус уже окрашен
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}

	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).
		Render(strings.Join(lines, "\n"))
}
