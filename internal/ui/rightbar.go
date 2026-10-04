// Rightbar — правая панель: состояние системы VPS (CPU, память,
// диск), каждая метрика — текущее значение + вертикальный бар-чарт
// истории последних замеров (см. sparkbar.go). Данные приходят от
// sysagent (см. sysclient.go) — независимого от collector сервиса,
// поэтому rightbar может показывать "нет данных" по системным
// метрикам, даже когда сам сборщик работает нормально (см. content),
// это два разных, намеренно не связанных друг с другом источника.
package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/sparkbar"
	"github.com/Dmitriy-495/dtui-kit/theme"
)

// warnAt/sosAt — единые пороги для всех трёх метрик rightbar: 70% —
// предупреждение, 90% — критично. Значения одинаковы для CPU/RAM/
// Disk ради простоты первой версии; если на практике окажется, что,
// например, для диска нужен другой порог (места на VPS 1TB, заполнение
// растёт медленно и предсказуемо, в отличие от всплесков CPU) — эти
// константы легко развести по отдельности позже.
const (
	metricWarnPercent = 70.0
	metricSOSPercent  = 90.0
)

// sparkbarWidth — сколько последних замеров показывать на графике.
// При SysPollInterval=5с это даёт историю на sparkbarWidth*5 секунд —
// 40 замеров ≈ 3.3 минуты, разумное окно для того, чтобы видеть
// недавний тренд, не растягивая rightbar по вертикали слишком сильно.
const sparkbarWidth = 40

var rightbarSectionTitleStyle = lipgloss.NewStyle().Foreground(theme.ColorData).Bold(true)

// renderRightbar строит содержимое правой панели. sysErr, если не
// nil, означает, что sysagent недоступен или вернул ошибку — тогда
// показываем причину вместо графиков (тот же принцип, что и m.lastErr
// для основного статус-соединения, но это независимый, отдельный
// источник данных — падение sysagent не должно влиять на content).
func renderRightbar(cpuHist, memHist, diskHist sparkbar.History, sysErr error, width, height int) string {
	innerWidth := width - 2 // рамка добавляет 2 символа, тот же паттерн, что в header/footer
	if innerWidth < 1 {
		innerWidth = 1
	}
	barWidth := innerWidth
	if barWidth > sparkbarWidth {
		barWidth = sparkbarWidth
	}

	var body string
	if sysErr != nil {
		body = theme.MutedStyle.Render(fmt.Sprintf("sysagent недоступен:\n%v", sysErr))
	} else {
		body = fmt.Sprintf(
			"%s\n%s\n\n%s\n%s\n\n%s\n%s",
			metricSectionTitle("CPU", cpuHist),
			sparkbar.RenderStyled(cpuHist, barWidth, metricWarnPercent, metricSOSPercent),
			metricSectionTitle("RAM", memHist),
			sparkbar.RenderStyled(memHist, barWidth, metricWarnPercent, metricSOSPercent),
			metricSectionTitle("DISK", diskHist),
			sparkbar.RenderStyled(diskHist, barWidth, metricWarnPercent, metricSOSPercent),
		)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Width(width).
		Height(height).
		Padding(1, 1).
		Render(body)
}

// metricSectionTitle — заголовок одной секции с текущим значением
// метрики, например "CPU  42.3%". "нет данных" вместо значения, пока
// история пуста (первый опрос sysagent ещё не завершился).
func metricSectionTitle(label string, hist sparkbar.History) string {
	title := rightbarSectionTitleStyle.Render(label)
	latest, ok := hist.Latest()
	if !ok {
		return title + "  " + theme.MutedStyle.Render("нет данных")
	}
	return title + "  " + theme.DataStyle.Render(fmt.Sprintf("%.1f%%", latest))
}
