// Проверка раскладки (флаг -check-layout): прогоняет зоны header и footer
// по диапазону ширин терминала и сообщает, где содержимое не помещается
// в строку (оно перенесётся и сломает рамку) или блоки стоят вплотную.
// Пороги show-from / show-until зависят от ширины шрифтов, а во внешнем
// layout.yaml их ничто не охраняет: этот режим закрывает такую дыру без
// запуска интерфейса и без подключения к сборщику.
package ui

import (
	"fmt"
	"io"
)

const (
	// Диапазон проверяемых ширин. Нижняя граница — минимальная ширина
	// терминала, на которой встроенная шапка помещается со счётчиком
	// снапшотов в 10 знаков (≈ 10 млрд: с запасом на годы сбора).
	checkMinWidth = 114
	checkMaxWidth = 260

	checkSnapshots = 9999999999
	checkStatus    = "подключено · аптайм сборщика 9999999с"
)

type zoneSample struct {
	name string
	// fit возвращает (нужно, есть, промежутков) для зоны на ширине w.
	fit func(w int) (need, avail, gaps int)
}

func checkSamples() []zoneSample {
	return []zoneSample{
		{"header", func(w int) (int, int, int) { return headerZone().Fit(w, headerData(checkSnapshots)) }},
		{"footer", func(w int) (int, int, int) { return footerZone().Fit(w, footerData(checkStatus)) }},
	}
}

// run — отрезок подряд идущих ширин с одним и тем же исходом.
type run struct {
	from, to int
	worst    int // наибольшая нехватка колонок (для ошибок)
}

// CheckLayout проверяет действующую раскладку (source — откуда она: путь
// к файлу или «встроенная») и пишет отчёт в w. Возвращает false, если
// есть ошибки (содержимое не помещается); «блоки вплотную» — только
// предупреждение.
func CheckLayout(w io.Writer, source string) bool {
	fmt.Fprintf(w, "Проверка раскладки (%s): ширины %d–%d, счётчик снапшотов — 10 знаков.\n",
		source, checkMinWidth, checkMaxWidth)

	ok := true
	for _, z := range checkSamples() {
		var overflow, touching []run
		minSlack, minAt := 1<<30, 0
		for width := checkMinWidth; width <= checkMaxWidth; width++ {
			need, avail, gaps := z.fit(width)
			slack := avail - need
			if slack < minSlack {
				minSlack, minAt = slack, width
			}
			switch {
			case slack < 0:
				overflow = addRun(overflow, width, -slack)
			case slack < gaps:
				touching = addRun(touching, width, 0)
			}
		}
		switch {
		case len(overflow) > 0:
			ok = false
			fmt.Fprintf(w, "%s: ОШИБКА — содержимое не помещается в строку и перенесётся:\n", z.name)
			for _, r := range overflow {
				fmt.Fprintf(w, "  ширины %s: не хватает до %d колонок\n", span(r), r.worst)
			}
		case len(touching) > 0:
			fmt.Fprintf(w, "%s: ВНИМАНИЕ — между блоками нет промежутка:\n", z.name)
			for _, r := range touching {
				fmt.Fprintf(w, "  ширины %s\n", span(r))
			}
		default:
			fmt.Fprintf(w, "%s: ок (минимальный запас по ширине — %d, на %d колонках)\n", z.name, minSlack, minAt)
		}
	}
	if ok {
		fmt.Fprintln(w, "Итог: раскладка помещается во всём диапазоне.")
	} else {
		fmt.Fprintln(w, "Итог: есть ошибки. Поправьте show-from / show-until или шрифты (font-size) в раскладке.")
	}
	return ok
}

// addRun добавляет ширину width к последнему отрезку, если он идёт
// вплотную, иначе начинает новый.
func addRun(runs []run, width, shortfall int) []run {
	if n := len(runs); n > 0 && runs[n-1].to == width-1 {
		runs[n-1].to = width
		runs[n-1].worst = max(runs[n-1].worst, shortfall)
		return runs
	}
	return append(runs, run{from: width, to: width, worst: shortfall})
}

func span(r run) string {
	if r.from == r.to {
		return fmt.Sprintf("%d", r.from)
	}
	return fmt.Sprintf("%d–%d", r.from, r.to)
}
