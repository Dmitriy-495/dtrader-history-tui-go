// Этот файл реализует HTTP-клиент к sysagent (см. dtrader-history/
// sysagent) — опрашивает GET /sys раз в SysPollInterval, шлёт
// разобранный ответ в канал Updates. В отличие от statusclient (WS,
// push-модель от сервера), здесь обычный поллинг: sysagent — простой
// REST-эндпоинт без постоянного соединения, TUI сам решает, как часто
// ему нужны свежие данные.
package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SysPollInterval — как часто опрашивать sysagent. Раз в 5 секунд —
// метрики CPU/RAM/Disk не меняются настолько резко, чтобы иметь смысл
// опрашивать их чаще (в отличие от статуса сборщика, где счётчик
// снапшотов растёт на 100мс-дельтах), а на графике из истории в
// несколько десятков точек это даёт разумное окно в несколько минут.
const SysPollInterval = 5 * time.Second

// sysRequestTimeout — таймаут одного HTTP-запроса к sysagent. Сам
// sysagent тратит ~200мс на выборку CPU (см. sysagent/server,
// cpuSampleInterval) плюс сетевая задержка — 3 секунды с большим
// запасом, чтобы медленная сеть не считалась мгновенно "недоступно".
const sysRequestTimeout = 3 * time.Second

// SysStatus — то же самое, что sysagent/server.SysStatus, продублировано
// здесь, чтобы TUI не тянул прямую зависимость от репозитория
// dtrader-history (два независимых репозитория/бинарника, общий
// протокол — тот же принцип, что и между collector и tui_client через
// statusclient.Status).
type SysStatus struct {
	CPUPercent      float64 `json:"cpu_percent"`
	MemTotalBytes   uint64  `json:"mem_total_bytes"`
	MemUsedBytes    uint64  `json:"mem_used_bytes"`
	MemUsedPercent  float64 `json:"mem_used_percent"`
	DiskTotalBytes  uint64  `json:"disk_total_bytes"`
	DiskUsedBytes   uint64  `json:"disk_used_bytes"`
	DiskFreeBytes   uint64  `json:"disk_free_bytes"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
}

// SysClient — клиент sysagent. Не имеет собственного цикла реконнекта
// со счётчиком поколений, как statusclient — здесь это не нужно: раз
// в SysPollInterval делается независимый HTTP-запрос, неудача одного
// запроса не требует восстановления состояния соединения, просто
// следующий запрос попробует снова.
type SysClient struct {
	baseURL    string
	token      string
	httpClient *http.Client

	Updates chan SysStatus
	Errors  chan error // буфер 1 с вытеснением устаревшего значения — тот же принцип, что и в statusclient.Client (см. client.go, sendError)

	stopCh    chan struct{}
	closeOnce func()
}

// NewSysClient создаёт клиента к sysagent на host:port.
func NewSysClient(host string, port int, token string) *SysClient {
	return &SysClient{
		baseURL:    fmt.Sprintf("http://%s:%d/sys", host, port),
		token:      token,
		httpClient: &http.Client{Timeout: sysRequestTimeout},
		Updates:    make(chan SysStatus),
		Errors:     make(chan error, 1),
		stopCh:     make(chan struct{}),
	}
}

// Stop останавливает цикл опроса.
func (c *SysClient) Stop() {
	select {
	case <-c.stopCh:
		// уже остановлен
	default:
		close(c.stopCh)
	}
}

// RunForever — цикл опроса раз в SysPollInterval. Возвращается только
// после Stop(). Предназначен для запуска в отдельной горутине.
func (c *SysClient) RunForever() {
	// Первый опрос сразу, не дожидаясь первого тика — тот же принцип,
	// что и в statusserver.pushLoop: не заставляем UI ждать до
	// SysPollInterval, чтобы увидеть хоть что-то.
	c.pollOnce()

	ticker := time.NewTicker(SysPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.pollOnce()
		}
	}
}

// pollOnce делает один HTTP-запрос и шлёт результат в Updates либо
// ошибку в Errors (см. sendError — тот же буфер-с-вытеснением, что и
// у statusclient.Client, по тем же причинам: важно последнее
// актуальное состояние, не история всех неудач поллинга).
func (c *SysClient) pollOnce() {
	status, err := c.fetch()
	if err != nil {
		c.sendError(err)
		return
	}
	select {
	case c.Updates <- status:
	case <-c.stopCh:
	}
}

func (c *SysClient) fetch() (SysStatus, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL, nil)
	if err != nil {
		return SysStatus{}, fmt.Errorf("sysagent: не удалось собрать запрос: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SysStatus{}, fmt.Errorf("sysagent: запрос не удался: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return SysStatus{}, fmt.Errorf("sysagent: неверный токен")
	}
	if resp.StatusCode != http.StatusOK {
		return SysStatus{}, fmt.Errorf("sysagent: HTTP %d", resp.StatusCode)
	}

	var status SysStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return SysStatus{}, fmt.Errorf("sysagent: не удалось разобрать ответ: %w", err)
	}
	return status, nil
}

// sendError — тот же буфер-с-вытеснением, что и в statusclient.Client
// (см. client.go для подробного обоснования: TUI важна только самая
// свежая ошибка, никогда не должен блокировать цикл опроса).
func (c *SysClient) sendError(err error) {
	select {
	case c.Errors <- err:
		return
	default:
	}
	select {
	case <-c.Errors:
	default:
	}
	select {
	case c.Errors <- err:
	default:
	}
}
