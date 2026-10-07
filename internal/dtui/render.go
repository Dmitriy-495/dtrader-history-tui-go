package dtui

import (
	"image/color"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

const (
	itemGap = 2 // пробелов между элементами одного кластера
	// горизонтальные потери ширины в зоне: рамка (2) + Padding(0, 2) (4)
	zoneInsets = 6
)

// Render отрисовывает зону в рамке theme.BorderStyle на ширину width.
func (z *Zone) Render(width int, data Data) string {
	avail := width - zoneInsets
	left, center, right := z.clusters(width, data)

	blocks := make([]string, 0, 3)
	for _, c := range [][]placed{left, center, right} {
		if len(c) > 0 {
			blocks = append(blocks, joinCluster(c))
		}
	}
	line := spreadBlocks(avail, blocks...)

	content := lipgloss.NewStyle().Padding(0, 2).Width(width - 2).Render(line)
	return theme.BorderStyle.Render(content)
}

// Height — высота зоны (с рамкой) при данной ширине и данных.
func (z *Zone) Height(width int, data Data) int {
	return lipgloss.Height(z.Render(width, data))
}

type placed struct {
	block  string
	valign string
}

func (z *Zone) clusters(width int, data Data) (left, center, right []placed) {
	for _, n := range z.Row {
		if !visible(n, width) {
			continue
		}
		p := placed{block: renderNode(n, width, data), valign: n.VAlign}
		switch n.Align {
		case "center":
			center = append(center, p)
		case "right":
			right = append(right, p)
		default:
			left = append(left, p)
		}
	}
	return
}

func visible(n Node, width int) bool {
	if n.ShowFrom > 0 && width < n.ShowFrom {
		return false
	}
	if n.ShowUntil > 0 && width > n.ShowUntil {
		return false
	}
	return true
}

// joinCluster склеивает элементы одного кластера слева направо, выравнивая
// каждый по вертикали (valign, по умолчанию по центру).
func joinCluster(items []placed) string {
	h := 0
	for _, it := range items {
		h = max(h, lipgloss.Height(it.block))
	}
	parts := make([]string, 0, len(items)*2)
	for i, it := range items {
		if i > 0 {
			parts = append(parts, strings.Repeat(" ", itemGap))
		}
		parts = append(parts, lipgloss.PlaceVertical(h, vpos(it.valign), it.block))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func vpos(s string) lipgloss.Position {
	switch s {
	case "top":
		return lipgloss.Top
	case "bottom":
		return lipgloss.Bottom
	}
	return lipgloss.Center
}

func hpos(s string) lipgloss.Position {
	switch s {
	case "center":
		return lipgloss.Center
	case "right":
		return lipgloss.Right
	}
	return lipgloss.Left
}

// spreadBlocks раскладывает блоки в одну строку, поровну распределяя
// свободное место avail между ними (по вертикали — по центру).
func spreadBlocks(avail int, blocks ...string) string {
	if len(blocks) == 1 {
		return blocks[0]
	}
	used := 0
	for _, b := range blocks {
		used += lipgloss.Width(b)
	}
	gaps := len(blocks) - 1
	pad := max(avail-used, 0)
	parts := make([]string, 0, len(blocks)*2)
	for i, b := range blocks {
		parts = append(parts, b)
		if i < gaps {
			gap := pad / gaps
			if i == gaps-1 {
				gap = pad - (pad/gaps)*(gaps-1)
			}
			parts = append(parts, strings.Repeat(" ", gap))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// renderNode отрисовывает один элемент или колонку.
func renderNode(n Node, width int, data Data) string {
	if len(n.Column) > 0 {
		return renderColumn(n, width, data)
	}
	var out string
	switch n.Type {
	case "banner":
		out = rect(styled(n, n.FontColor).Render(renderBanner(n.Text, fontOf(n, banner.FontStandard))))
	case "clock":
		out = rect(styled(n, n.FontColor).Render(renderClock(data[n.Bind], fontOf(n, banner.FontSmall))))
	case "text":
		out = renderText(n, data)
	}
	if n.MarginTop > 0 {
		out = strings.Repeat("\n", n.MarginTop) + out
	}
	return out
}

func renderColumn(n Node, width int, data Data) string {
	var blocks []string
	var aligns []string
	w := 0
	for _, c := range n.Column {
		if !visible(c, width) {
			continue
		}
		b := renderNode(c, width, data)
		blocks = append(blocks, b)
		aligns = append(aligns, c.Align)
		w = max(w, lipgloss.Width(b))
	}
	for i := range blocks {
		blocks[i] = lipgloss.PlaceHorizontal(w, hpos(aligns[i]), blocks[i])
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func renderText(n Node, data Data) string {
	value, ok := data[n.Bind]
	if !ok {
		value = "?"
	}
	labelColor := n.LabelColor
	if labelColor == "" {
		labelColor = "muted"
	}
	lbl := lipgloss.NewStyle().Foreground(colorOf(labelColor))
	out := styled(n, n.FontColor).Render(value)
	// Пустые подпись/суффикс не оборачиваем в стиль: иначе в выводе
	// остаются пустые ANSI-отрезки.
	if n.Label != "" {
		out = lbl.Render(n.Label) + out
	}
	if n.Suffix != "" {
		out += lbl.Render(n.Suffix)
	}
	return out
}

// styled строит стиль узла: цвет и жирность.
func styled(n Node, colorName string) lipgloss.Style {
	s := lipgloss.NewStyle().Bold(n.Bold)
	if c := colorOf(colorName); c != nil {
		s = s.Foreground(c)
	}
	return s
}

// colorOf переводит смысловое имя палитры (theme) или код цвета в цвет.
func colorOf(name string) color.Color {
	switch name {
	case "":
		return nil
	case "brand":
		return theme.ColorBorder
	case "ok":
		return theme.ColorOK
	case "data":
		return theme.ColorData
	case "muted":
		return theme.ColorMuted
	case "warn":
		return theme.ColorWarn
	case "sos":
		return theme.ColorSOS
	}
	return lipgloss.Color(name)
}

// fontOf выбирает шрифт: явный font приоритетнее font-size, иначе def.
func fontOf(n Node, def banner.Font) banner.Font {
	if n.Font != "" {
		return banner.Font(n.Font)
	}
	if f, ok := sizeFonts[n.FontSize]; ok {
		return f
	}
	return def
}

func renderBanner(text string, f banner.Font) string {
	return strings.Trim(banner.MustRender(text, banner.Options{Font: f}), "\n")
}

// rect выравнивает ширину всех строк блока: нужно, чтобы выравнивание и
// склейка не искажали форму FIGlet-фигур.
func rect(block string) string {
	return lipgloss.JoinVertical(lipgloss.Left, block)
}

// Часы: каждый знак рисуется в ячейке фиксированной ширины, иначе блок
// дёргается влево-вправо при смене цифр (у цифр в FIGlet разная ширина).
var (
	glyphMu    sync.Mutex
	glyphCache = map[banner.Font]map[rune]string{}
)

func glyphsFor(f banner.Font) map[rune]string {
	glyphMu.Lock()
	defer glyphMu.Unlock()
	if g, ok := glyphCache[f]; ok {
		return g
	}
	render := func(r rune) string { return renderBanner(string(r), f) }
	cell := 0
	for r := '0'; r <= '9'; r++ {
		cell = max(cell, lipgloss.Width(render(r)))
	}
	g := make(map[rune]string, 11)
	for r := '0'; r <= '9'; r++ {
		g[r] = lipgloss.NewStyle().Width(cell).Render(render(r))
	}
	colon := render(':')
	g[':'] = lipgloss.NewStyle().Width(lipgloss.Width(colon)).Render(colon)
	glyphCache[f] = g
	return g
}

// renderClock собирает строку вида "07:06:34" из глифов фиксированной ширины.
func renderClock(text string, f banner.Font) string {
	g := glyphsFor(f)
	parts := make([]string, 0, len(text))
	for _, r := range text {
		if glyph, ok := g[r]; ok {
			parts = append(parts, glyph)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
