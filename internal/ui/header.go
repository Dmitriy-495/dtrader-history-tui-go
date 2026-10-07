// Этот файл реализует верхнюю панель TUI: заголовок слева (FIGlet-лого
// и подпись), справа крупные часы UTC со счётчиком снапшотов под ними;
// на узком терминале — прежняя однострочная раскладка. Внешний вид
// описан в default_layout.yaml (или во внешнем layout.yaml) и
// отрисовывается движком internal/dtui с компонентами dtui-kit —
// раскладка шапки в этом файле больше не зашита в Go.
package ui

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/dtui"
)

//go:embed default_layout.yaml
var defaultLayoutYAML []byte

// layout — действующая раскладка: встроенная, пока не вызван LoadLayout.
var layout = mustParseLayout(defaultLayoutYAML)

func mustParseLayout(raw []byte) *dtui.Layout {
	l, err := dtui.Parse(raw)
	if err != nil {
		panic("встроенная раскладка default_layout.yaml некорректна: " + err.Error())
	}
	if _, ok := l.Zone("header"); !ok {
		panic("во встроенной раскладке нет зоны header")
	}
	return l
}

// LoadLayout заменяет встроенную раскладку содержимым файла path. Ошибка
// разбора (неизвестный ключ, тип, шрифт и т.п.) возвращается как есть.
func LoadLayout(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	l, err := dtui.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if _, ok := l.Zone("header"); !ok {
		return fmt.Errorf("%s: нет зоны header", path)
	}
	layout = l
	return nil
}

func headerZone() *dtui.Zone {
	z, _ := layout.Zone("header")
	return z
}

// headerData — значения, на которые ссылаются элементы шапки (bind).
func headerData(totalSnapshots int64) dtui.Data {
	return dtui.Data{
		"time_utc":        time.Now().UTC().Format("15:04:05"),
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
