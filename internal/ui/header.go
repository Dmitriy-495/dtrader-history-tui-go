// Этот файл реализует верхнюю панель TUI: заголовок слева, время
// биржи по UTC по центру, общее количество снапшотов справа —
// раскладка и арифметика ширины взяты из dtrader-tui-6/internal/tui/
// header.go (раздел 11 CHECKPOINT.md dtrader-6, единая дизайн-система
// проектов dtrader), не изобретены заново — та версия уже прошла
// через несколько раундов правок на реальных багах (перенос строки
// из-за неверного расчёта ширины, смещение рамки), которые нет смысла
// наступать повторно здесь.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Dmitriy-495/dtui-kit/theme"
)

var headerBrandStyle = lipgloss.NewStyle().Foreground(theme.ColorBorder).Bold(true)

// renderHeader строит содержимое шапки. totalSnapshots — сумма
// snapshots_since_start по всем символам (см. buildTotalSnapshots),
// 0 до первого полученного статуса — это легитимное значение, не
// требует отдельного состояния "нет данных" в отличие от статуса
// отдельных символов (см. rightbar.go), потому что 0 здесь означает
// "сборщик только что запущен", не "данные отсутствуют".
//
// width — полная ширина терминала. См. подробный комментарий в
// оригинале (dtrader-tui-6/header.go) про арифметику .Width()/.Padding()
// и зачем нужен headerSafetyMargin — тот же паттерн применён здесь
// без изменений.
func renderHeader(totalSnapshots int64, width int) string {
	left := headerBrandStyle.Render("⚡ dtrader-history") + "  " + theme.MutedStyle.Render("сборщик order book")
	center := theme.DataStyle.Render(time.Now().UTC().Format("15:04:05")) + theme.MutedStyle.Render(" UTC")
	right := theme.MutedStyle.Render("снапшотов всего: ") + theme.DataStyle.Bold(true).Render(fmt.Sprintf("%d", totalSnapshots))

	textWidth := width - 2 // см. header.go оригинала — рамка добавляет ровно 2 символа

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	centerWidth := lipgloss.Width(center)

	const headerSafetyMargin = 4 // запас на случай расхождения lipgloss.Width() с реальной шириной эмодзи в терминале (см. оригинал)

	totalPad := textWidth - headerSafetyMargin - leftWidth - rightWidth - centerWidth
	if totalPad < 0 {
		totalPad = 0
	}
	leftGap := totalPad / 2
	rightGap := totalPad - leftGap

	line := left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right

	content := lipgloss.NewStyle().Padding(0, 2).Width(textWidth).Render(line)
	return theme.BorderStyle.Render(content)
}
