package dtui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/banner"
)

func TestParse_RejectsMistakes(t *testing.T) {
	cases := map[string]string{
		"неизвестный ключ":    "layout:\n  - zone: h\n    row:\n      - { type: text, bind: x, colour: ok }\n",
		"неизвестный type":    "layout:\n  - zone: h\n    row:\n      - { type: picture, bind: x }\n",
		"неизвестный размер":  "layout:\n  - zone: h\n    row:\n      - { type: banner, text: a, font-size: xl }\n",
		"неизвестный шрифт":   "layout:\n  - zone: h\n    row:\n      - { type: banner, text: a, font: no-such-font }\n",
		"неизвестный align":   "layout:\n  - zone: h\n    row:\n      - { type: text, bind: x, align: middle }\n",
		"banner без text":     "layout:\n  - zone: h\n    row:\n      - { type: banner }\n",
		"clock без bind":      "layout:\n  - zone: h\n    row:\n      - { type: clock }\n",
		"зона без имени":      "layout:\n  - row:\n      - { type: text, bind: x }\n",
		"колонка в колонке":   "layout:\n  - zone: h\n    row:\n      - column:\n          - column:\n              - { type: text, bind: x }\n",
		"digits не у clock":   "layout:\n  - zone: h\n    row:\n      - { type: banner, text: a, font: digits }\n",
		"отрицательный порог": "layout:\n  - zone: h\n    row:\n      - { type: text, bind: x, show-from: -1 }\n",
	}
	for name, raw := range cases {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("%s: ожидалась ошибка разбора", name)
		}
	}
}

func TestParse_AcceptsValidLayout(t *testing.T) {
	raw := "layout:\n  - zone: h\n    row:\n      - { type: banner, text: ab, font-size: sm, font-color: brand }\n      - column:\n          - { type: clock, bind: t }\n          - { type: text, bind: n, margin-top: 1 }\n"
	l, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := l.Zone("h"); !ok {
		t.Error("зона h должна находиться по имени")
	}
	if _, ok := l.Zone("nope"); ok {
		t.Error("несуществующая зона не должна находиться")
	}
}

// Ширина часов не должна зависеть от того, какие цифры на экране
// (иначе блок дёргается при смене секунд).
func TestRenderClock_ConstantSize(t *testing.T) {
	wantW, wantH := -1, -1
	for _, hms := range []string{"00:00:00", "11:11:11", "23:59:59", "08:18:08", "12:34:56"} {
		if _, err := time.Parse("15:04:05", hms); err != nil {
			t.Fatal(err)
		}
		got := renderClock(hms, banner.FontSmall)
		w, h := lipgloss.Width(got), lipgloss.Height(got)
		if wantW < 0 {
			wantW, wantH = w, h
		}
		if w != wantW || h != wantH {
			t.Errorf("%s: размер часов %dx%d, want %dx%d (часы дёргаются)", hms, w, h, wantW, wantH)
		}
	}
}

func TestVisibility_ShowFromShowUntil(t *testing.T) {
	n := Node{ShowFrom: 100, ShowUntil: 150}
	for w, want := range map[int]bool{99: false, 100: true, 150: true, 151: false} {
		if got := visible(n, w); got != want {
			t.Errorf("visible(width=%d) = %v, want %v", w, got, want)
		}
	}
	if !visible(Node{}, 1) {
		t.Error("без порогов элемент виден всегда")
	}
}

func TestRender_MissingBindShowsQuestionMark(t *testing.T) {
	l, err := Parse([]byte("layout:\n  - zone: h\n    row:\n      - { type: text, label: \"v=\", bind: nope }\n"))
	if err != nil {
		t.Fatal(err)
	}
	z, _ := l.Zone("h")
	if out := z.Render(40, Data{}); !strings.Contains(out, "v=") || !strings.Contains(out, "?") {
		t.Errorf("при отсутствии значения ожидался '?', получено:\n%s", out)
	}
}

// clock с font: digits принимается разбором и рисует «ЧЧ:ММ» без секунд.
func TestRenderClock_Digits(t *testing.T) {
	raw := "layout:\n  - zone: h\n    row:\n      - { type: clock, bind: t, font: digits }\n"
	if _, err := Parse([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	n := Node{Type: "clock", Bind: "t", Font: "digits"}
	at := func(ns int) string {
		ts := time.Date(2026, 10, 8, 7, 6, 34, ns, time.UTC)
		return clockText(n, Data{"t": ts.Format(time.RFC3339Nano)})
	}
	on, off := at(100_000_000), at(700_000_000)
	if lipgloss.Height(on) != 3 {
		t.Errorf("высота часов %d, want 3", lipgloss.Height(on))
	}
	if lipgloss.Width(on) != lipgloss.Width(off) {
		t.Errorf("ширина мигающих часов меняется: %d и %d", lipgloss.Width(on), lipgloss.Width(off))
	}
	if on == off {
		t.Error("двоеточие должно мигать: вид в первой и второй половине секунды совпал")
	}
}

// Минуты меняют картинку, секунды — нет (часы без секунд).
func TestRenderClock_DigitsIgnoreSeconds(t *testing.T) {
	n := Node{Type: "clock", Bind: "t", Font: "digits"}
	f := func(sec int) string {
		ts := time.Date(2026, 10, 8, 7, 6, sec, 100_000_000, time.UTC)
		return clockText(n, Data{"t": ts.Format(time.RFC3339Nano)})
	}
	if f(5) != f(55) {
		t.Error("секунды не должны влиять на вид часов")
	}
}

// Нет значения или оно битое — прочерки той же ширины, без паники.
func TestRenderClock_DigitsFallback(t *testing.T) {
	n := Node{Type: "clock", Bind: "t", Font: "digits"}
	good := clockText(n, Data{"t": time.Now().UTC().Format(time.RFC3339Nano)})
	for _, d := range []Data{{}, {"t": "мусор"}} {
		got := clockText(n, d)
		if lipgloss.Width(got) != lipgloss.Width(good) || lipgloss.Height(got) != 3 {
			t.Errorf("прочерки %v: размер %dx%d, want ширина %d, высота 3", d, lipgloss.Width(got), lipgloss.Height(got), lipgloss.Width(good))
		}
	}
}

// Fit: нужно — сумма ширин видимых блоков, есть — ширина минус рамка и
// отступы, промежутков на один меньше, чем блоков.
func TestZoneFit(t *testing.T) {
	raw := "layout:\n  - zone: h\n    row:\n      - { type: text, label: \"abcd\", bind: x }\n      - { type: text, label: \"efg\", bind: x, align: right }\n"
	l, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	z, _ := l.Zone("h")
	need, avail, gaps := z.Fit(40, Data{"x": "12"})
	if need != 6+5 || avail != 40-zoneInsets || gaps != 1 {
		t.Errorf("Fit = (%d,%d,%d), want (11,%d,1)", need, avail, gaps, 40-zoneInsets)
	}
	// Оба элемента подставляют значение x: на 5 знаков длиннее — каждый шире на 5.
	if need, _, _ := z.Fit(40, Data{"x": "1234567"}); need != 11+10 {
		t.Errorf("длиннее значение — шире блок: need=%d, want 21", need)
	}
}
