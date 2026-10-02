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

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

var headerBrandStyle = lipgloss.NewStyle().Foreground(theme.ColorBorder).Bold(true)

// headerLogo — FIGlet-логотип (шрифт small, 4 строки). ...
var headerLogo = strings.Trim(banner.Render("dtrader", banner.Options{Font: "banner3"}), "\n")

// headerHeight — полная высота шапки: строки логотипа + верх и низ
// рамки. Единый источник для renderHeader и Model.bodyHeight.
var headerHeight = lipgloss.Height(headerLogo) + 2

// renderHeader строит содержимое шапки. totalSnapshots — сумма
// snapshots_since_start по всем символам (см. buildTotalSnapshots),
// 0 до первого полученного статуса — это легитимное значение, не
// требует отдельного состояния "нет данных" в отличие от статуса
// отдельных символов (см. rightbar.go), потому что 0 здесь означает
// "сборщик только что запущен", не "данные отсутствуют".
//
// width — полная ширина терминала. Арифметика ширины (рамка, паддинг)
// — тот же паттерн, что в dtrader-tui-6/header.go, см. комментарии
// внутри. Подпись "history" скрывается, если не хватает места.
func renderHeader(totalSnapshots int64, width int) string {
	logo := headerBrandStyle.Render(headerLogo)
	center := theme.DataStyle.Render(time.Now().UTC().Format("15:04:05")) + theme.MutedStyle.Render(" UTC")
	right := theme.MutedStyle.Render("снапшотов всего: ") + theme.DataStyle.Bold(true).Render(fmt.Sprintf("%d", totalSnapshots))

	textWidth := width - 2 // см. header.go оригинала — рамка добавляет ровно 2 символа

	// Width() в lipgloss включает Padding(0, 2) ниже, поэтому реально
	// доступно на 4 символа меньше, чем textWidth. (В оригинале эта же
	// константа называлась headerSafetyMargin и объяснялась шириной
	// эмодзи — на деле она компенсировала именно паддинг.)
	const headerHorizontalPadding = 4

	// Подпись рядом с логотипом показываем, только если после неё
	// остаётся хотя бы по одному пробелу между тремя блоками; на узком
	// терминале она первой уступает место остальному.
	left := logo
	tagline := lipgloss.JoinHorizontal(lipgloss.Center, logo, "  ", theme.MutedStyle.Render("history"))
	free := func(l string) int {
		return textWidth - headerHorizontalPadding - lipgloss.Width(l) - lipgloss.Width(center) - lipgloss.Width(right)
	}
	if free(tagline) >= 2 {
		left = tagline
	}
	totalPad := free(left)
	if totalPad < 0 {
		totalPad = 0
	}
	leftGap := totalPad / 2
	rightGap := totalPad - leftGap

	line := lipgloss.JoinHorizontal(lipgloss.Center,
		left, strings.Repeat(" ", leftGap), center, strings.Repeat(" ", rightGap), right)

	content := lipgloss.NewStyle().Padding(0, 2).Width(textWidth).Render(line)
	return theme.BorderStyle.Render(content)
}
