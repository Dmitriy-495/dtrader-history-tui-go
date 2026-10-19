// Rightbar — правая панель: состояние системы VPS (CPU, память, диск).
// Каждая метрика — две строки: имя (CPU, RAM, DISK) тем же крупным шрифтом
// dtui-kit/banner, что и имена монет в таблице (m.tickerFont; без него —
// обычным текстом), и под ним строка «текущее значение + график истории
// последних замеров» (sparkbar). Данные приходят от sysagent (см.
// sysclient.go) — независимого от collector сервиса, поэтому rightbar может
// показывать «нет данных» по системным метрикам, даже когда сам сборщик
// работает нормально (см. content), это два разных, намеренно не связанных
// друг с другом источника.
package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/sparkbar"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

// warnAt/sosAt — единые пороги для всех трёх метрик rightbar: 70% —
// предупреждение, 90% — критично. Значения одинаковы для CPU/RAM/
// Disk ради простоты первой версии; если на практике окажется, что,
// например, для диска нужен другой порог (места на VPS 1TB, заполнение
// растёт медленно и предсказуемо, в отличие от всплесков CPU) — эти
// константы легко развести по отдельности позже.
const (
	metricWarnPercent = 70.0
	metricSOSPercent  = 90.0
)

// sparkbarWidth — сколько последних замеров показывать на графике.
// При SysPollInterval=5с это даёт историю на sparkbarWidth*5 секунд —
// 40 замеров ≈ 3.3 минуты, разумное окно для того, чтобы видеть
// недавний тренд, не растягивая rightbar по вертикали слишком сильно.
const sparkbarWidth = 40

const (
	// rightbarInsets — сколько строк и столбцов забирают рамка (2) и
	// Padding(1,1) (2): на содержимое остаётся size-4.
	rightbarInsets = 4
	// metricValueW — ширина колонки значения («100.0%»), чтобы графики всех
	// трёх метрик начинались с одного столбца.
	metricValueW = 6
	// metricGap — пробелов между значением и графиком.
	metricGap = 2
	// metricSectionGap — пустых строк между секциями.
	metricSectionGap = 1
)

var rightbarSectionTitleStyle = lipgloss.NewStyle().Foreground(theme.ColorData).Bold(true)

// metric — одна секция rightbar.
type metric struct {
	name string
	hist sparkbar.History
}

// renderRightbar строит правую панель width × height. sysErr, если не nil,
// означает, что sysagent недоступен или вернул ошибку — тогда показываем
// причину вместо графиков (тот же принцип, что и m.lastErr для основного
// статус-соединения, но это независимый, отдельный источник данных —
// падение sysagent не должно влиять на content). font — шрифт крупных имён
// ("" — обычный текст).
func renderRightbar(cpuHist, memHist, diskHist sparkbar.History, sysErr error, font banner.Font, width, height int) string {
	innerW := max(width-rightbarInsets, 1)
	innerH := max(height-rightbarInsets, 1)

	var body string
	if sysErr != nil {
		body = theme.MutedStyle.Render(fmt.Sprintf("sysagent недоступен:\n%v", sysErr))
	} else {
		body = metricSections([]metric{{"CPU", cpuHist}, {"RAM", memHist}, {"DISK", diskHist}}, font, innerW, innerH)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Width(width).
		Height(height).
		Padding(1, 1).
		Render(body)
}

// metricSections рисует все секции: имя, под ним строка значения с графиком,
// между секциями пустая строка. Имена крупные, пока они помещаются по ширине и
// все секции целиком по высоте; иначе имена заменяются обычным текстом, а если
// не помещается и он — лишние нижние строки обрезаются (панель не должна
// вылезать за свою рамку).
func metricSections(metrics []metric, font banner.Font, innerW, innerH int) string {
	names := make([]string, len(metrics))
	big := font != ""
	if big {
		figW, figH := 0, 0
		for i, m := range metrics {
			names[i] = strings.TrimRight(banner.Render(m.name, banner.Options{Font: font}), "\n")
			figW = max(figW, lipgloss.Width(names[i]))
			figH = max(figH, lipgloss.Height(names[i]))
		}
		need := len(metrics)*(figH+1) + (len(metrics)-1)*metricSectionGap
		big = figW <= innerW && need <= innerH
	}
	if !big {
		for i, m := range metrics {
			names[i] = rightbarSectionTitleStyle.Render(m.name)
		}
	}

	barW := min(max(innerW-metricValueW-metricGap, 1), sparkbarWidth)
	var lines []string
	for i, m := range metrics {
		if i > 0 {
			for g := 0; g < metricSectionGap; g++ {
				lines = append(lines, "")
			}
		}
		if big {
			names[i] = theme.DataStyle.Render(names[i])
		}
		lines = append(lines, names[i], metricValueLine(m.hist, barW))
	}

	joined := strings.Join(lines, "\n")
	if rows := strings.Split(joined, "\n"); len(rows) > innerH {
		joined = strings.Join(rows[:innerH], "\n")
	}
	return joined
}

// metricValueLine — «значение + график» одной строкой: текущее значение
// метрики (например, «42.3%», «--», пока история пуста и первый опрос
// sysagent ещё не завершился) в колонке фиксированной ширины и график.
func metricValueLine(hist sparkbar.History, barWidth int) string {
	value := theme.MutedStyle.Render("--")
	if latest, ok := hist.Latest(); ok {
		value = theme.DataStyle.Render(fmt.Sprintf("%.1f%%", latest))
	}
	value = lipgloss.NewStyle().Width(metricValueW).Render(value)
	return value + strings.Repeat(" ", metricGap) +
		sparkbar.RenderStyled(hist, barWidth, metricWarnPercent, metricSOSPercent)
}
