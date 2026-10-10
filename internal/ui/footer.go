// Footer — нижняя панель: статусная строка (состояние подключения) и
// подсказка по выходу (suffix в раскладке). Внешний вид описан зоной footer в
// default_layout.yaml (или во внешнем layout.yaml) и отрисовывается тем же
// движком, что и шапка. Цвет статуса зависит от состояния и выбирается в
// Go (см. Model.footerStatus): приложение отдаёт движку уже окрашенную
// строку status.
//
// В будущем возможна командная строка по аналогии с vim (Shift+:) — не
// реализовано, только заложено местом (см. Model.Update, case
// tea.KeyPressMsg).
package ui

import "github.com/Dmitriy-495/dtrader-history-tui/internal/dtui"

func footerZone() *dtui.Zone { return zoneOf("footer") }

// footerData — значения, на которые ссылаются элементы подвала (bind).
// status — уже окрашенный текст состояния подключения. Подсказка по
// выходу ("q — выход") задана в раскладке (suffix), а не в Go.
func footerData(status string) dtui.Data {
	return dtui.Data{"status": status}
}

// renderFooter рисует нижнюю панель на всю ширину терминала.
func renderFooter(status string, width int) string {
	return footerZone().Render(width, footerData(status))
}

// footerHeightAt — высота подвала (с рамкой) при данной ширине и статусе:
// по факту отрисовки, потому что раскладка подвала внешняя и может быть
// выше одной строки, а длинный статус переносится.
func footerHeightAt(status string, width int) int {
	return footerZone().Height(width, footerData(status))
}
