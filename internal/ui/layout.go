// Раскладка приложения: описание зон (header, footer) в YAML и доступ к
// ним. Встроенная раскладка вшита в бинарник; внешний файл (флаг
// -layout) заменяет те зоны, которые в нём описаны, остальные берутся из
// встроенной, так что файл, написанный только для шапки, продолжает
// работать, когда появляются новые зоны.
package ui

import (
	_ "embed"
	"fmt"
	"os"
	"slices"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/dtui"
)

//go:embed default_layout.yaml
var defaultLayoutYAML []byte

// knownZones — зоны, которые приложение умеет рисовать движком. Остальные
// (тело, правая панель) пока рисуются Go-кодом.
var knownZones = []string{"header", "footer"}

// defaultLayout — встроенная раскладка; layout — действующая: встроенная,
// пока не вызван LoadLayout.
var (
	defaultLayout = mustParseLayout(defaultLayoutYAML)
	layout        = defaultLayout
)

func mustParseLayout(raw []byte) *dtui.Layout {
	l, err := dtui.Parse(raw)
	if err != nil {
		panic("встроенная раскладка default_layout.yaml некорректна: " + err.Error())
	}
	for _, name := range knownZones {
		if _, ok := l.Zone(name); !ok {
			panic("во встроенной раскладке нет зоны " + name)
		}
	}
	return l
}

// LoadLayout заменяет зоны встроенной раскладки теми, что описаны в файле
// path. Ошибка разбора (неизвестный ключ, тип, шрифт и т.п.), неизвестное
// или повторяющееся имя зоны, пустой файл — ошибки при старте.
func LoadLayout(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	l, err := dtui.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(l.Zones) == 0 {
		return fmt.Errorf("%s: не описано ни одной зоны (%v)", path, knownZones)
	}
	seen := map[string]bool{}
	for _, z := range l.Zones {
		if !slices.Contains(knownZones, z.Name) {
			return fmt.Errorf("%s: неизвестная зона %q (%v)", path, z.Name, knownZones)
		}
		if seen[z.Name] {
			return fmt.Errorf("%s: зона %q описана дважды", path, z.Name)
		}
		seen[z.Name] = true
	}
	layout = l
	return nil
}

// zoneOf возвращает зону из действующей раскладки, а если её там нет —
// из встроенной.
func zoneOf(name string) *dtui.Zone {
	if z, ok := layout.Zone(name); ok {
		return z
	}
	z, _ := defaultLayout.Zone(name)
	return z
}
