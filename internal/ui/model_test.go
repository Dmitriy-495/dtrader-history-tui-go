package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

// newTestModel — та же сигнатура, что и New, но с фиксированным
// sysPort, чтобы не повторять один и тот же аргумент в каждом тесте.
func newTestModel(host string, port int, token string, symbols []string) Model {
	return New(host, port, token, symbols, 8766)
}

// TestUpdate_StatusMsgMarksConnected проверяет, что получение
// statusMsg переводит модель в состояние "подключено" и заполняет
// таблицу строками для всех сконфигурированных символов.
func TestUpdate_StatusMsgMarksConnected(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT", "SOL_USDT"})

	status := statusclient.Status{
		Type:           "status",
		UptimeSeconds:  100,
		TotalDiskBytes: 2048,
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 42, LastSnapshotAgeSecs: 0.5},
		},
	}

	updated, _ := m.Update(statusMsg(status))
	mm := updated.(Model)

	if !mm.connected {
		t.Error("connected должен стать true после получения statusMsg")
	}
	if mm.lastStatus == nil {
		t.Fatal("lastStatus не должен быть nil после получения statusMsg")
	}
	if mm.lastStatus.UptimeSeconds != 100 {
		t.Errorf("UptimeSeconds = %f, want 100", mm.lastStatus.UptimeSeconds)
	}

	rows := mm.table.Rows()
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (по числу сконфигурированных символов)", len(rows))
	}
}

// TestBuildRows_SymbolWithoutDataShowsNoData — символ, сконфигурированный
// в CLI, но отсутствующий в статус-сообщении, должен показать "нет
// данных", а не пропасть из таблицы или запаниковать.
func TestBuildRows_SymbolWithoutDataShowsNoData(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT", "UNKNOWN_SYMBOL"})
	status := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 5, LastSnapshotAgeSecs: 1.0},
		},
	}
	m.lastStatus = &status
	m.lastStatusAt = time.Now()

	rows := m.buildRows()
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	// Порядок строк должен совпадать с порядком в m.symbols, не с
	// порядком ключей карты (карты в Go не гарантируют порядок).
	if rows[0][0] != "BTC_USDT" {
		t.Errorf("rows[0][0] = %q, want BTC_USDT (порядок должен совпадать с конфигурацией)", rows[0][0])
	}
	if rows[1][0] != "UNKNOWN_SYMBOL" {
		t.Errorf("rows[1][0] = %q, want UNKNOWN_SYMBOL", rows[1][0])
	}
	if !strings.Contains(rows[1][3], "нет данных") {
		t.Errorf("rows[1][3] = %q, want содержит 'нет данных'", rows[1][3])
	}
}

// TestBuildRows_StaleThresholdMarksStuck проверяет порог "застрял" —
// идентичный Python-версии STALE_THRESHOLD_SECONDS = 5.0.
func TestBuildRows_StaleThresholdMarksStuck(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})

	// Свежая запись (0.5с) — должна быть "работает".
	fresh := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 1, LastSnapshotAgeSecs: 0.5},
		},
	}
	m.lastStatus = &fresh
	m.lastStatusAt = time.Now()
	rows := m.buildRows()
	if !strings.Contains(rows[0][3], "работает") {
		t.Errorf("свежая запись (0.5с): rows[0][3] = %q, want содержит 'работает'", rows[0][3])
	}

	// Устаревшая запись (10с > порога 5с) — должна быть "ЗАСТРЯЛ?".
	stale := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 1, LastSnapshotAgeSecs: 10.0},
		},
	}
	m.lastStatus = &stale
	m.lastStatusAt = time.Now()
	rows = m.buildRows()
	if !strings.Contains(rows[0][3], "ЗАСТРЯЛ") {
		t.Errorf("устаревшая запись (10с): rows[0][3] = %q, want содержит 'ЗАСТРЯЛ'", rows[0][3])
	}
}

// TestBuildRows_NegativeAgeMeansNoDataYet проверяет сигнал "ещё нет
// данных" (-1 от статус-сервера, см. storage.Writer.GetStatus) — это
// тоже должно считаться "нет данных"/"застрял", не крашить форматирование.
func TestBuildRows_NegativeAgeMeansNoDataYet(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	status := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 0, LastSnapshotAgeSecs: -1},
		},
	}
	m.lastStatus = &status
	m.lastStatusAt = time.Now()

	rows := m.buildRows()
	if rows[0][2] != "—" {
		t.Errorf("age text = %q, want %q при LastSnapshotAgeSecs=-1", rows[0][2], "—")
	}
	if !strings.Contains(rows[0][3], "ЗАСТРЯЛ") {
		t.Errorf("статус = %q, want содержит ЗАСТРЯЛ при отсутствии данных", rows[0][3])
	}
}

// TestTotalSnapshots_SumsAcrossSymbols проверяет расчёт общего
// счётчика для правого блока header.
func TestTotalSnapshots_SumsAcrossSymbols(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT", "SOL_USDT"})
	if m.totalSnapshots() != 0 {
		t.Errorf("totalSnapshots() до первого статуса = %d, want 0", m.totalSnapshots())
	}

	status := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 100},
			"SOL_USDT": {SnapshotsSinceStart: 50},
		},
	}
	m.lastStatus = &status
	if m.totalSnapshots() != 150 {
		t.Errorf("totalSnapshots() = %d, want 150", m.totalSnapshots())
	}
}

// TestUpdate_QuitsOnQKey проверяет биндинг выхода (q), аналог
// BINDINGS = [("q", "quit", ...)] в app.py.
func TestUpdate_QuitsOnQKey(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("ожидалась команда (tea.Quit) при нажатии q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("команда вернула %T, want tea.QuitMsg", msg)
	}
}

// TestUpdate_ClientErrMarksDisconnected проверяет, что ошибка от
// статус-клиента (например, неверный токен) корректно отражается в
// состоянии модели.
func TestUpdate_ClientErrMarksDisconnected(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	m.connected = true

	updated, _ := m.Update(clientErrMsg{err: errTest})
	mm := updated.(Model)

	if mm.connected {
		t.Error("connected должен стать false после ошибки клиента")
	}
	if mm.lastErr == nil {
		t.Error("lastErr должен быть заполнен")
	}
}

// TestUpdate_SysStatusMsgUpdatesHistories проверяет, что sysStatusMsg
// пушит значения в соответствующие MetricHistory (CPU/RAM/Disk).
func TestUpdate_SysStatusMsgUpdatesHistories(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})

	sys := SysStatus{CPUPercent: 42.5, MemUsedPercent: 60.1, DiskUsedPercent: 80.9}
	updated, _ := m.Update(sysStatusMsg(sys))
	mm := updated.(Model)

	if !mm.sysConnected {
		t.Error("sysConnected должен стать true после sysStatusMsg")
	}
	cpu, ok := mm.cpuHist.Latest()
	if !ok || cpu != 42.5 {
		t.Errorf("cpuHist.Latest() = (%f, %v), want (42.5, true)", cpu, ok)
	}
	mem, ok := mm.memHist.Latest()
	if !ok || mem != 60.1 {
		t.Errorf("memHist.Latest() = (%f, %v), want (60.1, true)", mem, ok)
	}
	disk, ok := mm.diskHist.Latest()
	if !ok || disk != 80.9 {
		t.Errorf("diskHist.Latest() = (%f, %v), want (80.9, true)", disk, ok)
	}
}

// TestUpdate_SysErrMsgMarksSysDisconnected проверяет обработку ошибки
// от sysagent — независимо от основного статус-соединения.
func TestUpdate_SysErrMsgMarksSysDisconnected(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	m.sysConnected = true

	updated, _ := m.Update(sysErrMsg{err: errTest})
	mm := updated.(Model)

	if mm.sysConnected {
		t.Error("sysConnected должен стать false после ошибки sysagent")
	}
	if mm.lastSysErr == nil {
		t.Error("lastSysErr должен быть заполнен")
	}
}

// TestUpdate_StatusMsgBeforeWindowSizeDoesNotPanic — регрессионный
// тест на реальную панику, найденную при прогоне остальных тестов:
// таблица bubbles/table паникует в SetRows(), если у неё вообще нет
// колонок (table.WithColumns() не была вызвана). Раньше колонки
// задавались только в resizeTable() (вызывается из обработки
// tea.WindowSizeMsg) — если statusMsg приходит раньше, чем bubbletea
// успевает прислать WindowSizeMsg (обе горутины запускаются в Init()
// конкурентно, порядок сообщений не гарантирован), buildRows()/
// SetRows() падали на таблице без единой колонки. New() теперь
// задаёт разумные колонки по умолчанию сразу, до любого resizeTable().
func TestUpdate_StatusMsgBeforeWindowSizeDoesNotPanic(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	// НЕ отправляем tea.WindowSizeMsg — сразу шлём statusMsg, как могло
	// бы прийти в реальности при неудачном порядке горутин.
	status := statusclient.Status{
		Symbols: map[string]statusclient.SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 1, LastSnapshotAgeSecs: 0.1},
		},
	}
	// Успех теста — то, что это не паникует.
	m.Update(statusMsg(status))
}

var errTest = &testError{"тестовая ошибка"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// TestView_RendersWithoutPanicBeforeWindowSize проверяет, что View()
// не паникует до первого tea.WindowSizeMsg (width == 0).
func TestView_RendersWithoutPanicBeforeWindowSize(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	out := m.View()
	if out == "" {
		t.Error("View() до WindowSizeMsg не должен возвращать пустую строку")
	}
}

// TestView_RendersWithoutPanicBeforeFirstStatus проверяет, что View()
// не паникует до получения первого статуса (lastStatus == nil), уже
// после получения размера окна — состояние "Подключение...".
func TestView_RendersWithoutPanicBeforeFirstStatus(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := updated.(Model).View()
	if !strings.Contains(out, "Подключение") {
		t.Errorf("View() до первого статуса должен содержать 'Подключение...', получено: %q", out)
	}
}

// TestView_RendersErrorState проверяет отображение ошибки в footer.
func TestView_RendersErrorState(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ := m.Update(clientErrMsg{err: errTest})
	out := updated.(Model).View()
	if !strings.Contains(out, "тестовая ошибка") {
		t.Errorf("View() при ошибке должен содержать текст ошибки, получено: %q", out)
	}
}

// TestView_RendersSysErrorInRightbar проверяет, что ошибка sysagent
// отображается именно в rightbar, а не теряется — независимый от
// основного статуса источник (см. renderRightbar).
func TestView_RendersSysErrorInRightbar(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	updated, _ := m.Update(sysErrMsg{err: errTest})
	out := updated.(Model).View()
	if !strings.Contains(out, "sysagent недоступен") {
		t.Errorf("View() при ошибке sysagent должен содержать 'sysagent недоступен', получено: %q", out)
	}
}

// update — маленький хелпер, чтобы работать с Model как со значением
// через цепочку Update-вызовов без повторения приведения типов в
// каждом тесте.
func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

// TestWindowSizeMsg_ResizesTable проверяет обработку изменения
// размера терминала — таблица должна подстроиться под ширину/высоту.
func TestWindowSizeMsg_ResizesTable(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	mm := updated.(Model)
	if mm.width != 100 || mm.height != 40 {
		t.Errorf("width/height = %d/%d, want 100/40", mm.width, mm.height)
	}
}

// TestContentRightbarWidth_SplitsEvenly проверяет деление 50/50 между
// content и rightbar — решение автора проекта, зафиксированное явно.
func TestContentRightbarWidth_SplitsEvenly(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	mm := updated.(Model)

	cw := mm.contentWidth()
	rw := mm.rightbarWidth()
	if cw+rw != 120 {
		t.Errorf("contentWidth()+rightbarWidth() = %d, want 120 (вся ширина должна быть использована)", cw+rw)
	}
	// Допускаем разницу в 1 символ из-за целочисленного деления
	// нечётной ширины — не обязано быть математически идеальным 50/50.
	diff := cw - rw
	if diff < -1 || diff > 1 {
		t.Errorf("contentWidth()=%d rightbarWidth()=%d — расхождение больше 1 символа, деление не похоже на 50/50", cw, rw)
	}
}

// TestView_ContentAndRightbarBordersAlignVertically — регрессионный
// тест на визуальный баг, найденный ручной проверкой рендера: рамки
// content-блока (таблица) и rightbar-блока (метрики) закрывались на
// РАЗНЫХ строках, потому что bubbles/table.View() включает собственную
// строку заголовка колонок сверх заданного SetHeight(N), а
// contentStyle.Height(N) считал N готовой высотой без поправки на эту
// скрытую дополнительную строку — тогда как у rightbar (без такого
// скрытого заголовка) расчёт совпадал точно. Итоговая высота всего
// View() (после исправления, см. resizeTable — tableHeaderRows) должна
// точно совпадать с заданной высотой терминала, а каждая строка -
// иметь одинаковую видимую ширину (без ANSI-кодов) по всему полотну,
// иначе рамки двух колонок неизбежно разъезжаются по вертикали.
func TestView_ContentAndRightbarBordersAlignVertically(t *testing.T) {
	m := newTestModel("localhost", 8765, "tok", []string{"BTC_USDT", "SOL_USDT", "AVAX_USDT"})
	const width, height = 140, 35
	m, _ = update(m, tea.WindowSizeMsg{Width: width, Height: height})

	out := m.View()
	lines := strings.Split(out, "\n")

	if len(lines) != height {
		t.Fatalf("View() дал %d строк, want %d (высота терминала)", len(lines), height)
	}

	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Errorf("строка %d: видимая ширина (lipgloss.Width) = %d, want %d — рамки, скорее всего, разъехались по вертикали где-то выше этой строки", i, got, width)
		}
	}
}
