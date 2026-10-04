// Footer — нижняя панель: пока статусная строка (состояние
// подключения + подсказка по выходу), в будущем возможна командная
// строка по аналогии с vim (Shift+:) — не реализовано сейчас, только
// заложено местом в архитектуре (см. Model.Update, case tea.KeyMsg:
// свободна для добавления обработки ":" без переписывания остального).
//
// Стиль и арифметика ширины — тот же паттерн, что и в header.go,
// портированный из dtrader-tui-6/internal/tui/footer.go.
package ui

import (
	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/theme"
)

// renderFooter рисует нижнюю панель на всю ширину терминала.
// statusLine — текст текущего статуса подключения (см. Model.footerStatusLine).
func renderFooter(statusLine string, width int) string {
	textWidth := width - 2
	content := lipgloss.NewStyle().Padding(0, 2).Width(textWidth).Render(statusLine)
	return theme.BorderStyle.Render(content)
}

// footerHints — подсказка по выходу, добавляется к статусной строке.
const footerHints = "q — выход"
