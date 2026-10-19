package ui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
	"github.com/Dmitriy-495/dtui-kit/sparkbar"
)

const sparkRunes = "▁▂▃▄▅▆▇█"

func histOf(values ...float64) sparkbar.History {
	h := sparkbar.NewHistory(sparkbarWidth)
	for _, v := range values {
		h.Push(v)
	}
	return h
}

// figLines — непустые строки рисунка name шрифтом font без цвета.
func figLines(name string, font banner.Font) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(plain(banner.Render(name, banner.Options{Font: font})), "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimRight(l, " "))
		}
	}
	return out
}

// Две строки на метрику: имя крупным шрифтом (как имена монет), под ним строка
// «значение + график». Слов CPU/RAM/DISK обычным текстом при этом нет.
func TestRenderRightbar_NameInFontThenValueWithGraph(t *testing.T) {
	cpu, mem, disk := histOf(10, 20, 42.5), histOf(60, 61, 60.1), histOf(80, 81, 80.9)
	s := plain(renderRightbar(cpu, mem, disk, nil, banner.FontBanner3Mini, 100, 30))
	rows := strings.Split(s, "\n")

	for _, name := range []string{"CPU", "RAM", "DISK"} {
		for _, l := range figLines(name, banner.FontBanner3Mini) {
			if !strings.Contains(s, l) {
				t.Fatalf("в панели нет строки рисунка %q (%s):\n%s", l, name, s)
			}
		}
		if strings.Contains(s, name) {
			t.Errorf("имя %s должно быть нарисовано шрифтом, а не текстом:\n%s", name, s)
		}
	}

	for _, want := range []string{"42.5%", "60.1%", "80.9%"} {
		idx := -1
		for i, r := range rows {
			if strings.Contains(r, want) {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("нет значения %s:\n%s", want, s)
		}
		if !strings.ContainsAny(rows[idx], sparkRunes) {
			t.Errorf("значение %s должно быть на одной строке с графиком: %q", want, rows[idx])
		}
	}

	// Порядок: рисунок CPU стоит выше своего значения, а оно выше рисунка RAM.
	last := func(name string) int {
		ls := figLines(name, banner.FontBanner3Mini)
		for i := len(rows) - 1; i >= 0; i-- {
			if strings.Contains(rows[i], ls[len(ls)-1]) {
				return i
			}
		}
		return -1
	}
	val := func(v string) int {
		for i, r := range rows {
			if strings.Contains(r, v) {
				return i
			}
		}
		return -1
	}
	if !(last("CPU") < val("42.5%") && val("42.5%") < last("RAM")) {
		t.Errorf("порядок «имя → значение с графиком → следующее имя» нарушен:\n%s", s)
	}
}

// Графики всех метрик начинаются с одного столбца (колонка значения
// фиксированной ширины).
func TestRenderRightbar_GraphsAlignedInOneColumn(t *testing.T) {
	cpu, mem, disk := histOf(5), histOf(100), histOf(33.3)
	rows := strings.Split(plain(renderRightbar(cpu, mem, disk, nil, banner.FontBanner3Mini, 100, 30)), "\n")
	var starts []int
	for _, r := range rows {
		if strings.Contains(r, "%") {
			starts = append(starts, strings.IndexAny(r, sparkRunes))
		}
	}
	if len(starts) != 3 || starts[0] != starts[1] || starts[1] != starts[2] {
		t.Errorf("графики должны начинаться с одного столбца, получено %v", starts)
	}
}

// Пока истории нет — «--» вместо значения (первый опрос ещё не завершился).
func TestRenderRightbar_NoDataShowsDashes(t *testing.T) {
	e := sparkbar.NewHistory(sparkbarWidth)
	s := plain(renderRightbar(e, e, e, nil, banner.FontBanner3Mini, 100, 30))
	if strings.Count(s, "--") < 3 {
		t.Errorf("ожидалось «--» у каждой метрики:\n%s", s)
	}
}

// Без шрифта (режим прежней таблицы) имена — обычный текст в той же
// двухстрочной раскладке.
func TestRenderRightbar_PlainNamesWithoutFont(t *testing.T) {
	s := plain(renderRightbar(histOf(1), histOf(2), histOf(3), nil, "", 80, 20))
	for _, want := range []string{"CPU", "RAM", "DISK"} {
		if !strings.Contains(s, want) {
			t.Errorf("нет имени %s обычным текстом:\n%s", want, s)
		}
	}
}

// Размер панели всегда ровно width × height: на любой ширине, на любой высоте
// (включая слишком низкую: имена становятся текстом, лишнее обрезается) и в
// обоих режимах.
func TestRenderRightbar_ExactSizeAlways(t *testing.T) {
	h := histOf(10, 50, 95)
	for _, font := range []banner.Font{banner.FontBanner3Mini, banner.FontBanner3, ""} {
		for _, hh := range []int{5, 8, 12, 17, 18, 20, 30, 50} {
			for _, w := range []int{14, 24, 40, 57, 90, 140} {
				s := renderRightbar(h, h, h, nil, font, w, hh)
				if lipgloss.Width(s) != w || lipgloss.Height(s) != hh {
					t.Fatalf("font=%q %dx%d: размер %dx%d", font, w, hh, lipgloss.Width(s), lipgloss.Height(s))
				}
			}
		}
	}
}

// Крупные имена, пока помещаются; иначе автоматически текст (низкая панель).
func TestRenderRightbar_FallsBackToTextWhenTooLow(t *testing.T) {
	h := histOf(10)
	tall := plain(renderRightbar(h, h, h, nil, banner.FontBanner3Mini, 100, 30))
	if strings.Contains(tall, "CPU") {
		t.Error("на высокой панели имена должны быть крупными")
	}
	low := plain(renderRightbar(h, h, h, nil, banner.FontBanner3Mini, 100, 12))
	if !strings.Contains(low, "CPU") {
		t.Errorf("на низкой панели имена должны стать текстом:\n%s", low)
	}
}

// Ошибка sysagent по-прежнему показывается вместо графиков.
func TestRenderRightbar_SysErrorStillShown(t *testing.T) {
	h := histOf(10)
	s := plain(renderRightbar(h, h, h, errors.New("нет route"), banner.FontBanner3Mini, 100, 30))
	if !strings.Contains(s, "sysagent недоступен") || strings.ContainsAny(s, sparkRunes) {
		t.Errorf("ожидалась причина вместо графиков:\n%s", s)
	}
}
