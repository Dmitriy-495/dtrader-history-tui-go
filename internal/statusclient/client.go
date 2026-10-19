// Этот файл — точный порт dtrader-history-tui (Python-версия),
// status_client.py: подключается к WS-серверу статуса collector'а,
// шлёт токен первым сообщением, вызывает callback при каждом
// полученном статус-сообщении, автоматически переподключается при
// обрыве связи. Собственно UI (TUI-модель) ничего не знает о деталях
// протокола — только получает уже готовую структуру Status через канал.
package statusclient

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultReconnectDelay — та же пауза, что в Python-версии
// (reconnect_delay_seconds=3.0 по умолчанию в status_client.py).
const DefaultReconnectDelay = 3 * time.Second

// SymbolStatus — статистика по одному символу, поля совпадают с
// протоколом статус-сервера (см. collector/statusserver/server.go).
type SymbolStatus struct {
	SnapshotsSinceStart int64   `json:"snapshots_since_start"`
	LastSnapshotAgeSecs float64 `json:"last_snapshot_age_seconds"`
}

// Status — сообщение, приходящее от статус-сервера при каждом push.
type Status struct {
	Type           string                  `json:"type"`
	UptimeSeconds  float64                 `json:"uptime_seconds"`
	TotalDiskBytes int64                   `json:"total_disk_bytes"`
	Symbols        map[string]SymbolStatus `json:"symbols"`
}

// authMessage — первое сообщение, отправляемое клиентом серверу.
type authMessage struct {
	Token string `json:"token"`
}

// Client подключается к ws://{host}:{port}, аутентифицируется
// токеном, шлёт полученные Status-сообщения в канал Updates.
// Автоматически переподключается при обрыве — тот же принцип, что и
// gateway.OrderBookCollector на стороне сборщика (см. dtrader-history).
type Client struct {
	host  string
	port  int
	token string

	reconnectDelay time.Duration

	Updates chan Status
	Errors  chan error // не фатальные ошибки/события реконнекта — для отображения в UI ("переподключение...")

	stopCh chan struct{}
}

// New создаёт Client. Updates — небуферизованный канал (читать должен
// вызывающий код, иначе Client будет заблокирован на отправке —
// каждое обновление статуса важно доставить, ни одно не должно
// потеряться). Errors — канал ёмкостью 1 с заменой устаревшего
// значения (см. sendError) — здесь, наоборот, важна только САМАЯ
// СВЕЖАЯ ошибка, а не очередь из всех подряд: TUI показывает "статус
// подключения прямо сейчас", а не журнал событий реконнекта, так что
// эта ошибка десятисекундной давности, которую ещё не успели прочитать,
// не должна задерживать показ следующей, более актуальной.
func New(host string, port int, token string) *Client {
	return &Client{
		host:           host,
		port:           port,
		token:          token,
		reconnectDelay: DefaultReconnectDelay,
		Updates:        make(chan Status),
		Errors:         make(chan error, 1),
		stopCh:         make(chan struct{}),
	}
}

// Stop останавливает цикл реконнекта. Безопасно вызывать один раз;
// повторный вызов паникует на закрытии уже закрытого канала — то же
// поведение, что и общепринято для стандартных Go-примитивов останова
// (в отличие от gateway.OrderBookCollector.Close(), которому явно
// нужна идемпотентность, здесь единственный вызывающий — UI при
// выходе, вызывается ровно один раз).
func (c *Client) Stop() {
	close(c.stopCh)
}

// RunForever — основной цикл: подключиться, аутентифицироваться,
// читать статус-сообщения, при обрыве — подождать и переподключиться.
// Возвращается только когда Stop() был вызван. Предназначен для
// запуска в отдельной горутине.
func (c *Client) RunForever() {
	wsURL := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", c.host, c.port)}

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.runOnce(wsURL.String()); err != nil {
			c.sendError(err)
		}

		select {
		case <-c.stopCh:
			return
		case <-time.After(c.reconnectDelay):
		}
	}
}

// sendError кладёт err в Errors, никогда не блокируясь и никогда не
// теряя единственное, что важно показать пользователю — самую свежую
// ошибку. Errors имеет ёмкость 1 (см. New). Если канал уже содержит
// непрочитанную ошибку с прошлой попытки реконнекта, она вытесняется
// новой: устаревшее сообщение об ошибке — то, что уже неактуально к
// моменту, когда UI до него доберётся, лучше вообще не увидеть, чем
// увидеть с опозданием как будто это текущее состояние.
//
// Раньше здесь был select с default-веткой прямо на отправке — а
// select с default никогда не блокируется, это его определяющее
// свойство, так что при небуферизованном канале это гарантированно
// теряло ошибку молча, если читатель не был готов ПРЯМО СЕЙЧАС (в
// реальном bubbletea-UI такое окно между обработкой одного
// clientErrMsg и повторным запуском чтения канала — ненулевое).
// Простая замена на блокирующий select без default чинила потерю
// данных, но создавала дедлок: если никто вообще не читает Errors
// (легитимный случай — не все потребители Client обязаны это делать),
// RunForever навсегда зависал на первой же ошибке, даже не доходя до
// повторного подключения. Буферизованный канал ёмкостью 1 с явным
// вытеснением устаревшего значения не теряет то единственное, что
// важно (последнее состояние), и никогда не блокирует реконнект-цикл.
func (c *Client) sendError(err error) {
	select {
	case c.Errors <- err:
		return
	default:
	}
	// Канал уже занят непрочитанной ошибкой — освобождаем место и
	// кладём свежую. Оба select здесь non-blocking: если между
	// вытеснением и повторной отправкой читатель успел забрать
	// значение сам (гонка с параллельным чтением), второй select
	// просто найдёт канал пустым и запись пройдёт как обычно; если
	// читатель успел прочитать именно в момент между <-c.Errors и
	// c.Errors <- err, канал снова свободен, запись тоже пройдёт.
	select {
	case <-c.Errors:
	default:
	}
	select {
	case c.Errors <- err:
	default:
		// Сюда попасть можно только при очень редкой гонке с другим
		// писателем в тот же момент — у Client ровно один писатель
		// (сам RunForever), так что практически недостижимо; не
		// блокируемся в любом случае.
	}
}

// runOnce — одна попытка подключения: коннект, аутентификация, чтение
// сообщений до обрыва. Возвращает ошибку, если соединение прервалось
// (в т.ч. штатно со стороны сервера, например code 4003 при неверном
// токене — это тоже репортится как ошибка, чтобы UI мог показать её
// пользователю явно, а не тихо зависать на "Подключение...").
func (c *Client) runOnce(wsURL string) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к %s: %w", wsURL, err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(authMessage{Token: c.token}); err != nil {
		return fmt.Errorf("не удалось отправить токен: %w", err)
	}

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("соединение с %s потеряно: %w", wsURL, err)
		}

		var status Status
		if err := json.Unmarshal(raw, &status); err != nil {
			// Некорректный JSON от сервера — пропускаем сообщение, не
			// рвём соединение (аналог "except json.JSONDecodeError:
			// continue" в Python-версии).
			continue
		}
		if status.Type != "status" {
			continue
		}

		select {
		case c.Updates <- status:
		case <-c.stopCh:
			return nil
		}
	}
}
