package ui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func plain(s string) string { return ansiRe.ReplaceAllString(s, "") }

// Главная защита порогов show-from / show-until в default_layout.yaml:
// на любой ширине от checkMinWidth колонок (со счётчиком снапшотов в 10
// знаков, как в -check-layout) шапка не должна ни переносить блоки на
// лишние строки, ни быть шире терминала. Пороги зависят от ширины
// шрифтов, и при смене шрифта в YAML этот тест сразу покажет, где они
// разошлись.
func TestRenderHeader_NoWrapAndExactWidthAcrossWidths(t *testing.T) {
	for w := checkMinWidth; w <= checkMaxWidth; w++ {
		h := renderHeader(checkSnapshots, w)
		if got := lipgloss.Width(h); got != w {
			t.Fatalf("width=%d: ширина шапки %d, want %d", w, got, w)
		}
		if got := lipgloss.Height(h); got != headerHeightAt(w) || got != 9 {
			t.Fatalf("width=%d: высота шапки %d, want 9\n%s", w, got, plain(h))
		}
	}
}

func TestRenderHeader_SubtitleOnlyWhenWide(t *testing.T) {
	const marker = "\\__, |" // хвост буквы y в подписи history (шрифт standard)
	if !strings.Contains(plain(renderHeader(1234567, 180)), marker) {
		t.Error("на 180 колонках подпись history (FIGlet) должна быть видна")
	}
	if strings.Contains(plain(renderHeader(1234567, 140)), marker) {
		t.Error("на 140 колонках подпись history должна быть скрыта")
	}
}

// Крупные часы со счётчиком под ними показываются на широком
// терминале; на узком остаётся прежняя однострочная раскладка.
func TestRenderHeader_BigClockOnlyWhenWide(t *testing.T) {
	wide := plain(renderHeader(1234567, 180))
	if !strings.Contains(wide, "UTC · снапшотов всего: 1234567") {
		t.Error("на 180 колонках счётчик должен стоять под крупными часами")
	}
	narrow := plain(renderHeader(1234567, 110))
	if strings.Contains(narrow, "UTC · снапшотов") {
		t.Error("на 110 колонках должна быть прежняя однострочная раскладка")
	}
	if !strings.Contains(narrow, "снапшотов всего: 1234567") || !strings.Contains(narrow, " UTC") {
		t.Error("на 110 колонках должны быть видны время UTC и счётчик")
	}
}

// Внешний layout.yaml действительно меняет шапку, а ошибка в нём не
// проходит молча.
func TestLoadLayout_OverridesAndRejectsTypos(t *testing.T) {
	saved := layout
	defer func() { layout = saved }()

	dir := t.TempDir()
	good := dir + "/good.yaml"
	bad := dir + "/bad.yaml"
	writeFile(t, good, "layout:\n  - zone: header\n    row:\n      - { type: text, label: \"HELLO \", bind: snapshots_total }\n")
	writeFile(t, bad, "layout:\n  - zone: header\n    row:\n      - { type: text, bind: snapshots_total, font-colour: ok }\n")

	if err := LoadLayout(bad); err == nil {
		t.Error("опечатка font-colour должна давать ошибку")
	}
	if err := LoadLayout(good); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain(renderHeader(42, 80)), "HELLO 42") {
		t.Error("внешняя раскладка должна заменить встроенную")
	}
}

// Внешний файл, описывающий только шапку, не ломает подвал: недостающие
// зоны берутся из встроенной раскладки.
func TestLoadLayout_MissingZoneFallsBackToDefault(t *testing.T) {
	saved := layout
	defer func() { layout = saved }()

	p := t.TempDir() + "/header_only.yaml"
	writeFile(t, p, "layout:\n  - zone: header\n    row:\n      - { type: text, label: \"HELLO \", bind: snapshots_total }\n")
	if err := LoadLayout(p); err != nil {
		t.Fatal(err)
	}
	f := plain(renderFooter("статус", 80))
	if !strings.Contains(f, "статус") || !strings.Contains(f, "q — выход") {
		t.Errorf("подвал должен остаться встроенным:\n%s", f)
	}
}

// Опечатка в имени зоны, повтор зоны и пустой файл — ошибки, а не
// молчаливый возврат к встроенной раскладке.
func TestLoadLayout_RejectsBadZones(t *testing.T) {
	saved := layout
	defer func() { layout = saved }()

	row := "    row:\n      - { type: text, bind: snapshots_total }\n"
	cases := map[string]string{
		"опечатка в имени зоны": "layout:\n  - zone: hedaer\n" + row,
		"зона дважды":           "layout:\n  - zone: header\n" + row + "  - zone: header\n" + row,
		"нет зон":               "layout: []\n",
	}
	dir := t.TempDir()
	for name, raw := range cases {
		p := dir + "/x.yaml"
		writeFile(t, p, raw)
		if err := LoadLayout(p); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

// Подвал: статус и подсказка в одной строке через три пробела, рамка на
// всю ширину; высота подвала считается по факту отрисовки.
func TestRenderFooter_StatusAndHint(t *testing.T) {
	for _, w := range []int{40, 80, 200} {
		f := renderFooter("подключено", w)
		if got := lipgloss.Width(f); got != w {
			t.Fatalf("width=%d: ширина подвала %d", w, got)
		}
		if got := footerHeightAt("подключено", w); got != 3 {
			t.Fatalf("width=%d: высота подвала %d, want 3", w, got)
		}
		if !strings.Contains(plain(f), "подключено   q — выход") {
			t.Errorf("width=%d: ожидались статус и подсказка через 3 пробела:\n%s", w, plain(f))
		}
	}
}

// Встроенная раскладка проходит собственную проверку (-check-layout).
func TestCheckLayout_BuiltinIsClean(t *testing.T) {
	var out strings.Builder
	if !CheckLayout(&out, "встроенная") {
		t.Fatalf("встроенная раскладка не прошла проверку:\n%s", out.String())
	}
	for _, want := range []string{"header: ок", "footer: ок", "Итог: раскладка помещается"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("в отчёте нет %q:\n%s", want, out.String())
		}
	}
}

// Раскладка с шрифтом, который не помещается, даёт ошибку с диапазоном
// ширин и величиной нехватки, а не молчит.
func TestCheckLayout_ReportsOverflow(t *testing.T) {
	saved := layout
	defer func() { layout = saved }()

	p := t.TempDir() + "/wide.yaml"
	writeFile(t, p, "layout:\n  - zone: header\n    row:\n      - { type: banner, text: dtrader, font-size: lg }\n      - { type: banner, text: history, font-size: lg }\n")
	if err := LoadLayout(p); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if CheckLayout(&out, p) {
		t.Fatalf("ожидалась ошибка проверки:\n%s", out.String())
	}
	for _, want := range []string{"header: ОШИБКА", "ширины 114–", "не хватает до", "footer: ок", "Итог: есть ошибки"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("в отчёте нет %q:\n%s", want, out.String())
		}
	}
}

// Блоки вплотную — предупреждение, а не ошибка: проверка проходит.
func TestCheckLayout_TouchingIsOnlyWarning(t *testing.T) {
	saved := layout
	defer func() { layout = saved }()

	// Баннер слева и текст справа ровно во всю ширину на самой узкой
	// проверяемой ширине: переполнения нет, но и промежутка между блоками.
	banner := "layout:\n  - zone: header\n    row:\n      - { type: banner, text: dtrader, font-size: lg }\n"
	p := t.TempDir() + "/touch.yaml"
	writeFile(t, p, banner)
	if err := LoadLayout(p); err != nil {
		t.Fatal(err)
	}
	need, avail, _ := headerZone().Fit(checkMinWidth, headerData(checkSnapshots))
	label := avail - need - len("9999999999") // остальное займёт значение счётчика
	if label < 1 {
		t.Skip("баннер не оставляет места под второй блок")
	}
	writeFile(t, p, banner+"      - { type: text, label: \""+strings.Repeat("x", label)+"\", bind: snapshots_total, align: right }\n")
	if err := LoadLayout(p); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if !CheckLayout(&out, p) {
		t.Fatalf("предупреждение не должно быть ошибкой:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "header: ВНИМАНИЕ") || !strings.Contains(out.String(), "ширины 114\n") {
		t.Fatalf("ожидалось предупреждение на ширине 114:\n%s", out.String())
	}
}
