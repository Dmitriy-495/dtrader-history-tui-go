// Bubbletea-модель TUI dtrader-history. Layout: header (заголовок,
// время биржи UTC, общий счётчик снапшотов) сверху, footer (статусная
// строка) снизу, между ними content (таблица символов, 50% ширины) и
// rightbar (состояние системы VPS — CPU/RAM/Disk с графиками, 50%
// ширины) — раскладка согласована с dtrader-tui-6 (см. header.go,
// footer.go, rightbar.go). Палитра и sparkbar-графики — из общей
// библиотеки github.com/Dmitriy-495/dtui-kit (theme, sparkbar), не
// локальные копии — см. dtui-kit/README.md.
package ui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Dmitriy-495/dtui-kit/sparkbar"
	"github.com/Dmitriy-495/dtui-kit/theme"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/statusclient"
)

// staleThreshold — порог "застрял", идентичен Python-версии (запас
// над частотой сбора данных 100мс, см. app.py STALE_THRESHOLD_SECONDS).
const staleThreshold = 5.0 * time.Second

// metricHistoryLen — сколько замеров CPU/RAM/Disk хранить для
// sparkbar-графиков в rightbar (см. sparkbar.go, sparkbar.History).
const metricHistoryLen = sparkbarWidth

// statusMsg оборачивает statusclient.Status для передачи через
// tea.Msg — bubbletea требует, чтобы сообщения из внешних источников
// (каналов) явно перекладывались в команды (см. waitForUpdate).
type statusMsg statusclient.Status

// clientErrMsg оборачивает ошибки статус-клиента (например,
// "неверный токен") для отображения в UI.
type clientErrMsg struct{ err error }

// sysStatusMsg и sysErrMsg — то же самое, что statusMsg/clientErrMsg,
// но для отдельного канала данных sysagent (CPU/RAM/Disk) — намеренно
// не смешаны с основным статус-протоколом, потому что это два разных,
// не связанных друг с другом источника (см. rightbar.go).
type sysStatusMsg SysStatus
type sysErrMsg struct{ err error }

// tickMsg используется для периодического обновления отображаемого
// "возраста" последнего снапшота между push-сообщениями сервера — без
// этого возраст в таблице обновлялся бы только раз в секунду вместе с
// push, а не выглядел бы "живым" между ними. Чисто косметическое
// отличие от Python-версии, не влияет на протокол.
type tickMsg time.Time

// Model — bubbletea-модель TUI.
type Model struct {
	client    *statusclient.Client
	sysClient *SysClient
	symbols   []string

	table table.Model

	connected    bool
	lastStatus   *statusclient.Status
	lastStatusAt time.Time
	lastErr      error

	sysConnected bool
	lastSysErr   error
	cpuHist      sparkbar.History
	memHist      sparkbar.History
	diskHist     sparkbar.History

	width, height int
}

// New создаёт Model. host/port/token/symbols — параметры основного
// статус-протокола (см. statusclient), как и раньше. sysagent
// подразумевается на том же host, порт sysPort (по умолчанию 8766,
// см. sysclient.go) — тот же VPS, соседний сервис.
func New(host string, port int, token string, symbols []string, sysPort int) Model {
	// Разумные значения по умолчанию, на случай если statusMsg придёт
	// раньше первого tea.WindowSizeMsg (реальный найденный баг: без
	// колонок вообще bubbles/table паникует на SetRows — таблица
	// должна быть валидной сразу после New(), не только после
	// первого resizeTable()). Как только придёт WindowSizeMsg,
	// resizeTable() пересчитает точную пропорциональную ширину (см.
	// tableColumns) — эти цифры лишь временная заглушка на самый
	// первый момент жизни программы.
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Символ", Width: 14},
			{Title: "Снапшотов", Width: 12},
			{Title: "Последняя запись", Width: 18},
			{Title: "Статус", Width: 12},
		}),
		table.WithFocused(true),
	)

	return Model{
		client:    statusclient.New(host, port, token),
		sysClient: NewSysClient(host, sysPort, token),
		symbols:   symbols,
		table:     t,
		cpuHist:   sparkbar.NewHistory(metricHistoryLen),
		memHist:   sparkbar.NewHistory(metricHistoryLen),
		diskHist:  sparkbar.NewHistory(metricHistoryLen),
	}
}

// Init запускает оба клиента (статус сборщика и sysagent) в фоне и
// начинает слушать их каналы.
func (m Model) Init() tea.Cmd {
	go m.client.RunForever()
	go m.sysClient.RunForever()
	return tea.Batch(
		waitForUpdate(m.client),
		waitForError(m.client),
		waitForSysUpdate(m.sysClient),
		waitForSysError(m.sysClient),
		tickCmd(),
	)
}

// waitForUpdate — tea.Cmd, блокирующийся на чтении из client.Updates;
// bubbletea вызывает его снова после каждого полученного сообщения
// (см. обработку statusMsg в Update), формируя постоянный поток
// обновлений из канала в Msg-цикл bubbletea.
func waitForUpdate(c *statusclient.Client) tea.Cmd {
	return func() tea.Msg {
		status, ok := <-c.Updates
		if !ok {
			return nil
		}
		return statusMsg(status)
	}
}

func waitForError(c *statusclient.Client) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-c.Errors
		if !ok {
			return nil
		}
		return clientErrMsg{err: err}
	}
}

func waitForSysUpdate(c *SysClient) tea.Cmd {
	return func() tea.Msg {
		status, ok := <-c.Updates
		if !ok {
			return nil
		}
		return sysStatusMsg(status)
	}
}

func waitForSysError(c *SysClient) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-c.Errors
		if !ok {
			return nil
		}
		return sysErrMsg{err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update обрабатывает входящие сообщения — Elm architecture:
// возвращает новую модель, а не мутирует состояние виджетов напрямую.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeTable()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.client.Stop()
			m.sysClient.Stop()
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case statusMsg:
		status := statusclient.Status(msg)
		m.connected = true
		m.lastErr = nil
		m.lastStatus = &status
		m.lastStatusAt = time.Now()
		m.table.SetRows(m.buildRows())
		return m, waitForUpdate(m.client)

	case clientErrMsg:
		m.connected = false
		m.lastErr = msg.err
		return m, waitForError(m.client)

	case sysStatusMsg:
		m.sysConnected = true
		m.lastSysErr = nil
		m.cpuHist.Push(msg.CPUPercent)
		m.memHist.Push(msg.MemUsedPercent)
		m.diskHist.Push(msg.DiskUsedPercent)
		return m, waitForSysUpdate(m.sysClient)

	case sysErrMsg:
		m.sysConnected = false
		m.lastSysErr = msg.err
		return m, waitForSysError(m.sysClient)

	case tickMsg:
		if m.lastStatus != nil {
			m.table.SetRows(m.buildRows())
		}
		return m, tickCmd()
	}

	return m, nil
}

// contentWidth/rightbarWidth — 50/50 деление ширины терминала между
// content (таблица) и rightbar (метрики системы), как решил автор
// проекта — если понадобится другая пропорция, менять только здесь.
func (m Model) contentWidth() int {
	return m.width / 2
}

func (m Model) rightbarWidth() int {
	return m.width - m.contentWidth()
}

// bodyHeight — высота средней части (content+rightbar) за вычетом
// header (headerHeightAt, см. header.go: по факту отрисовки) и
// footer (3 строки: верх рамки, контент, низ рамки) — тот же принцип, что и в dtrader-tui-6/app.go.
func (m Model) bodyHeight() int {
	const footerHeight = 3
	h := m.height - headerHeightAt(m.width) - footerHeight
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) resizeTable() {
	m.table.SetColumns(m.tableColumns())
	// В bubbles v2 table.SetHeight учитывает строку заголовка колонок
	// сам (в v1 нужна была ручная компенсация -1), а рамку contentStyle
	// (она входит в Height, см. render) мы вычитаем здесь: 2 строки.
	// В bubbles v2 у таблицы есть собственный viewport, и при нулевой
	// ширине (значение по умолчанию) строки данных не рисуются — виден
	// только заголовок. Ширина внутри рамки contentStyle: contentWidth - 2.
	m.table.SetWidth(m.contentWidth() - 2)
	m.table.SetHeight(m.bodyHeight() - 2)
}

// tableColumns пересчитывает ширину колонок таблицы пропорционально
// реальной ширине content-блока (см. contentWidth) — фиксированные
// ширины колонок (изначально рассчитанные под таблицу на всю ширину
// терминала) не помещались в новую половину экрана после перехода на
// раскладку 50/50 content/rightbar: сумма фиксированных ширин
// превышала contentWidth(), из-за чего bubbles/table переносил
// заголовок на две строки и ломал расчёт высоты в resizeTable (см.
// комментарий про tableHeaderRows) — найдено визуальной проверкой
// реального рендера. Пропорции (2:3:3:2 условных доли между символом/
// счётчиком/возрастом/статусом) подобраны так, чтобы под самые длинные
// реалистичные значения в каждой колонке ("AVAX_USDT", шестизначные
// счётчики, "123.4с назад", "ЗАСТРЯЛ?") оставался разумный запас на
// любой ширине терминала, а не только на той, что была в исходном
// фиксированном расчёте.
func (m Model) tableColumns() []table.Column {
	// Каждая колонка bubbles/table добавляет к своей Width по 1 символу
	// паддинга слева и справа (4 колонки = 8 символов), плюс рамка
	// contentStyle забирает ещё 2 символа от contentWidth. Раньше здесь
	// было 6: таблица получалась на 4 символа шире рамки, и rightbar
	// выезжал за правый край терминала (только при включённых цветах —
	// без них lipgloss обрезал лишнее, поэтому тесты этого не видели).
	const columnBorderOverhead = 10
	available := m.contentWidth() - columnBorderOverhead
	if available < 20 {
		available = 20 // минимум, при котором таблица ещё читаема; на совсем узких терминалах будет обрезаться самим bubbles/table, не паниковать
	}

	widths := proportionalWidths(available, []int{2, 3, 3, 2})
	return []table.Column{
		{Title: "Символ", Width: widths[0]},
		{Title: "Снапшотов", Width: widths[1]},
		{Title: "Последняя запись", Width: widths[2]},
		{Title: "Статус", Width: widths[3]},
	}
}

// proportionalWidths делит total между len(ratios) колонками
// пропорционально их весам в ratios, гарантируя, что сумма результата
// точно равна total (последняя колонка забирает остаток от целочисленного
// деления, а не теряет его — иначе сумма колонок могла бы оказаться
// на несколько символов меньше available и оставлять некрасивый
// пробел справа от таблицы).
func proportionalWidths(total int, ratios []int) []int {
	sumRatios := 0
	for _, r := range ratios {
		sumRatios += r
	}
	widths := make([]int, len(ratios))
	used := 0
	for i, r := range ratios {
		if i == len(ratios)-1 {
			widths[i] = total - used
			continue
		}
		w := total * r / sumRatios
		widths[i] = w
		used += w
	}
	return widths
}

// buildRows — аналог цикла "for symbol in self.collector_symbols" в
// исходной Python-версии: строит строки таблицы в ФИКСИРОВАННОМ
// порядке символов из конфигурации, не в порядке, в котором они
// встретились в последнем статус-сообщении (символ без данных всё
// равно должен появиться в таблице как "нет данных").
func (m Model) buildRows() []table.Row {
	rows := make([]table.Row, 0, len(m.symbols))
	for _, symbol := range m.symbols {
		if m.lastStatus == nil {
			rows = append(rows, table.Row{symbol, "0", "—", theme.MutedStyle.Render("нет данных")})
			continue
		}
		symStatus, ok := m.lastStatus.Symbols[symbol]
		if !ok {
			rows = append(rows, table.Row{symbol, "0", "—", theme.MutedStyle.Render("нет данных")})
			continue
		}

		// age_seconds пересчитывается локально от момента получения
		// push-сообщения (lastStatusAt), а не остаётся статичным
		// значением из самого сообщения — иначе цифра "застывала бы"
		// между push'ами раз в секунду, что выглядело бы как зависший
		// интерфейс, а не просто редкие обновления с сервера.
		baseAge := symStatus.LastSnapshotAgeSecs
		var ageText string
		var isStale bool
		if baseAge < 0 {
			ageText = "—"
			isStale = true
		} else {
			elapsedSincePush := time.Since(m.lastStatusAt).Seconds()
			age := baseAge + elapsedSincePush
			ageText = fmt.Sprintf("%.1fс назад", age)
			isStale = time.Duration(age*float64(time.Second)) > staleThreshold
		}

		statusText := theme.OKStyle.Render("работает")
		if isStale {
			statusText = theme.SOSStyle.Render("ЗАСТРЯЛ?")
		}

		rows = append(rows, table.Row{
			symbol,
			fmt.Sprintf("%d", symStatus.SnapshotsSinceStart),
			ageText,
			statusText,
		})
	}
	return rows
}

// totalSnapshots суммирует snapshots_since_start по всем символам —
// для правого блока заголовка (см. header.go).
func (m Model) totalSnapshots() int64 {
	if m.lastStatus == nil {
		return 0
	}
	var total int64
	for _, s := range m.lastStatus.Symbols {
		total += s.SnapshotsSinceStart
	}
	return total
}

// footerStatusLine строит текст статусной строки footer — состояние
// подключения к статус-серверу сборщика (основной источник, критичный
// для работы TUI) плюс подсказка по выходу. Состояние sysagent сюда
// намеренно не выводится — оно уже видно прямо в rightbar (см.
// renderRightbar), дублировать в footer было бы избыточно.
func (m Model) footerStatusLine() string {
	var status string
	switch {
	case m.lastErr != nil:
		status = theme.SOSStyle.Render(fmt.Sprintf("⚠ %v — переподключение...", m.lastErr))
	case m.lastStatus == nil:
		status = theme.MutedStyle.Render("Подключение...")
	default:
		uptime := int(m.lastStatus.UptimeSeconds + time.Since(m.lastStatusAt).Seconds())
		status = theme.OKStyle.Render(fmt.Sprintf("подключено · аптайм сборщика %dс", uptime))
	}
	return status + "   " + theme.MutedStyle.Render(footerHints)
}

// View рендерит: header, тело (content+rightbar через
// JoinHorizontal), footer — тот же паттерн сборки, что и в
// dtrader-tui-6/app.go (View(), JoinVertical(header, body, footer)).
// View — обёртка для bubbletea v2: контент строит render(), а
// альтернативный экран включается полем View (раньше — tea.WithAltScreen()).
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.width == 0 {
		// Первый кадр до получения tea.WindowSizeMsg — bubbletea
		// гарантированно пришлёт его почти сразу, но до этого момента
		// нет смысла пытаться считать раскладку от нулевой ширины.
		return "Инициализация..."
	}

	header := renderHeader(m.totalSnapshots(), m.width)

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorder).
		Width(m.contentWidth()).
		Height(m.bodyHeight())
	content := contentStyle.Render(m.table.View())

	rightbar := renderRightbar(m.cpuHist, m.memHist, m.diskHist, m.lastSysErr, m.rightbarWidth(), m.bodyHeight())

	body := lipgloss.JoinHorizontal(lipgloss.Top, content, rightbar)

	footer := renderFooter(m.footerStatusLine(), m.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
