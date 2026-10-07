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
// на любой ширине от 110 колонок шапка не должна ни переносить блоки на
// лишние строки, ни быть шире терминала. Пороги зависят от ширины
// шрифтов, и при смене шрифта в YAML этот тест сразу покажет, где они
// разошлись.
func TestRenderHeader_NoWrapAndExactWidthAcrossWidths(t *testing.T) {
	for w := 110; w <= 260; w++ {
		h := renderHeader(1234567, w)
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
