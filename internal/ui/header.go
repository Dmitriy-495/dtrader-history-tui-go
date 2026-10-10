// Этот файл реализует верхнюю панель TUI: заголовок слева (FIGlet-лого
// и подпись), справа крупные семисегментные часы UTC (ЧЧ:ММ) со счётчиком
// снапшотов под ними; на узком терминале — прежняя однострочная
// раскладка. Внешний вид описан зоной header в default_layout.yaml (или
// во внешнем layout.yaml, см. layout.go) и отрисовывается движком
// internal/dtui с компонентами dtui-kit — раскладка шапки в Go не зашита.
package ui

import (
	"fmt"
	"time"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/dtui"
)

func headerZone() *dtui.Zone { return zoneOf("header") }

// headerData — значения, на которые ссылаются элементы шапки (bind).
func headerData(totalSnapshots int64) dtui.Data {
	now := time.Now().UTC()
	return dtui.Data{
		"time_utc":        now.Format("15:04:05"),
		"now_utc":         now.Format(time.RFC3339Nano), // для clock с font: digits
		"snapshots_total": fmt.Sprintf("%d", totalSnapshots),
	}
}

// renderHeader строит шапку на ширину width. totalSnapshots — сумма
// snapshots_since_start по всем символам (см. buildTotalSnapshots), 0 до
// первого полученного статуса — легитимное значение ("сборщик только что
// запущен"), а не отсутствие данных.
func renderHeader(totalSnapshots int64, width int) string {
	return headerZone().Render(width, headerData(totalSnapshots))
}

// headerHeightAt — высота шапки (с рамкой) при данной ширине терминала.
// Зависит от ширины, потому что часть элементов скрывается на узком
// терминале, поэтому считается по факту отрисовки.
func headerHeightAt(width int) int {
	return headerZone().Height(width, headerData(0))
}
