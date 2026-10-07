// Package dtui — минимальный layout-движок: читает описание зон и
// элементов из YAML и отрисовывает их с помощью компонентов dtui-kit.
//
// Пакет сознательно НЕ импортирует код приложения: данные приходят через
// Data по именам (bind), так что при появлении второго потребителя пакет
// переносится в отдельный проект без правок (см. ARCHITECTURE.md).
//
// Бюджет сложности: строка, колонка (вертикальный стек из элементов),
// выравнивание, пороги ширины. Всё условное остаётся в Go приложения.
package dtui

import (
	"fmt"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/goccy/go-yaml"
)

// Node — элемент строки или колонки. Тип определяется полями: у обычного
// элемента задан Type, у колонки — Column.
type Node struct {
	Type       string `yaml:"type"`        // banner | clock | text
	Text       string `yaml:"text"`        // banner: что рисовать
	Bind       string `yaml:"bind"`        // clock, text: имя значения из Data
	Label      string `yaml:"label"`       // text: подпись перед значением
	Suffix     string `yaml:"suffix"`      // text: подпись после значения
	Font       string `yaml:"font"`        // явный FIGlet-шрифт (приоритетнее font-size)
	FontSize   string `yaml:"font-size"`   // sm | md | lg
	FontColor  string `yaml:"font-color"`  // brand|ok|data|muted|warn|sos или код цвета
	LabelColor string `yaml:"label-color"` // цвет подписей text (по умолчанию muted)
	Bold       bool   `yaml:"bold"`
	MarginTop  int    `yaml:"margin-top"` // пустых строк сверху элемента
	Align      string `yaml:"align"`      // left|center|right: в строке — кластер, в колонке — по горизонтали
	VAlign     string `yaml:"valign"`     // top|center|bottom: по вертикали внутри строки
	ShowFrom   int    `yaml:"show-from"`  // показывать при ширине терминала >= N
	ShowUntil  int    `yaml:"show-until"` // показывать при ширине терминала <= N
	Column     []Node `yaml:"column"`     // вертикальный стек элементов
}

// Zone — именованная зона (header, footer, ...) с одной строкой элементов.
type Zone struct {
	Name string `yaml:"zone"`
	Row  []Node `yaml:"row"`
}

// Layout — всё дерево.
type Layout struct {
	Zones []Zone `yaml:"layout"`
}

// Data — значения, которые приложение отдаёт элементам по именам (bind).
type Data map[string]string

// Шкала font-size → шрифт. Ступеней три: плавного масштаба в консоли нет.
var sizeFonts = map[string]banner.Font{
	"sm": banner.FontSmall,
	"md": banner.FontStandard,
	"lg": banner.FontBanner3,
}

// Parse читает YAML строго: неизвестный ключ, тип, размер, шрифт или
// выравнивание — это ошибка при старте, а не молчаливое игнорирование.
func Parse(raw []byte) (*Layout, error) {
	var l Layout
	if err := yaml.UnmarshalWithOptions(raw, &l, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("layout: %w", err)
	}
	for _, z := range l.Zones {
		if z.Name == "" {
			return nil, fmt.Errorf("layout: у зоны не задано имя (zone)")
		}
		for i, n := range z.Row {
			if err := validate(n, false); err != nil {
				return nil, fmt.Errorf("layout: зона %q, элемент %d: %w", z.Name, i+1, err)
			}
		}
	}
	return &l, nil
}

// Zone возвращает зону по имени.
func (l *Layout) Zone(name string) (*Zone, bool) {
	for i := range l.Zones {
		if l.Zones[i].Name == name {
			return &l.Zones[i], true
		}
	}
	return nil, false
}

func validate(n Node, inColumn bool) error {
	if len(n.Column) > 0 {
		if inColumn {
			return fmt.Errorf("колонка внутри колонки не поддерживается")
		}
		if n.Type != "" {
			return fmt.Errorf("у колонки не должно быть type (%q)", n.Type)
		}
		for i, c := range n.Column {
			if err := validate(c, true); err != nil {
				return fmt.Errorf("колонка, элемент %d: %w", i+1, err)
			}
		}
	} else {
		switch n.Type {
		case "banner":
			if n.Text == "" {
				return fmt.Errorf("banner: не задан text")
			}
		case "clock", "text":
			if n.Bind == "" {
				return fmt.Errorf("%s: не задан bind", n.Type)
			}
		case "":
			return fmt.Errorf("не задан ни type, ни column")
		default:
			return fmt.Errorf("неизвестный type %q (banner, clock, text)", n.Type)
		}
	}
	if n.FontSize != "" {
		if _, ok := sizeFonts[n.FontSize]; !ok {
			return fmt.Errorf("неизвестный font-size %q (sm, md, lg)", n.FontSize)
		}
	}
	if n.Font != "" && !banner.IsKnownFont(banner.Font(n.Font)) {
		return fmt.Errorf("неизвестный font %q (проверенные: %v)", n.Font, banner.Fonts())
	}
	switch n.Align {
	case "", "left", "center", "right":
	default:
		return fmt.Errorf("неизвестный align %q (left, center, right)", n.Align)
	}
	switch n.VAlign {
	case "", "top", "center", "bottom":
	default:
		return fmt.Errorf("неизвестный valign %q (top, center, bottom)", n.VAlign)
	}
	if n.MarginTop < 0 || n.ShowFrom < 0 || n.ShowUntil < 0 {
		return fmt.Errorf("margin-top, show-from и show-until не могут быть отрицательными")
	}
	return nil
}
