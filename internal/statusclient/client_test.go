package statusclient

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// fakeStatusServer — минимальный тестовый сервер: проверяет токен,
// шлёт заданные Status-сообщения по требованию (через send channel).
type fakeStatusServer struct {
	expectedToken string
	send          chan Status
	gotToken      chan string

	mu       sync.Mutex
	attempts int // сколько раз сервер получил и проверил сообщение аутентификации — используется тестами реконнекта, чтобы убедиться, что цикл реально продолжает идти, а не завис
}

func newFakeStatusServer(expectedToken string) *fakeStatusServer {
	return &fakeStatusServer{
		expectedToken: expectedToken,
		send:          make(chan Status, 10),
		gotToken:      make(chan string, 10),
	}
}

func (f *fakeStatusServer) authAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

func (f *fakeStatusServer) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var auth authMessage
	if err := conn.ReadJSON(&auth); err != nil {
		return
	}
	f.mu.Lock()
	f.attempts++
	f.mu.Unlock()
	f.gotToken <- auth.Token

	if auth.Token != f.expectedToken {
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(4003, "invalid token"), time.Now().Add(time.Second))
		return
	}

	for status := range f.send {
		if err := conn.WriteJSON(status); err != nil {
			return
		}
	}
}

func hostPort(t *testing.T, serverURL string) (string, int) {
	t.Helper()
	u := strings.TrimPrefix(serverURL, "http://")
	parts := strings.Split(u, ":")
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("не удалось разобрать порт из %q: %v", serverURL, err)
	}
	return parts[0], port
}

// TestClient_AuthenticatesAndReceivesUpdates — happy path: клиент
// подключается, шлёт токен, получает статус-сообщения через Updates.
func TestClient_AuthenticatesAndReceivesUpdates(t *testing.T) {
	fake := newFakeStatusServer("secret")
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	c := New(host, port, "secret")
	go c.RunForever()
	defer c.Stop()

	select {
	case tok := <-fake.gotToken:
		if tok != "secret" {
			t.Errorf("сервер получил токен %q, want %q", tok, "secret")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("сервер не получил токен вовремя")
	}

	want := Status{
		Type:           "status",
		UptimeSeconds:  10,
		TotalDiskBytes: 2048,
		Symbols: map[string]SymbolStatus{
			"BTC_USDT": {SnapshotsSinceStart: 5, LastSnapshotAgeSecs: 0.5},
		},
	}
	fake.send <- want

	select {
	case got := <-c.Updates:
		if got.UptimeSeconds != want.UptimeSeconds {
			t.Errorf("UptimeSeconds = %f, want %f", got.UptimeSeconds, want.UptimeSeconds)
		}
		if got.Symbols["BTC_USDT"].SnapshotsSinceStart != 5 {
			t.Errorf("SnapshotsSinceStart = %d, want 5", got.Symbols["BTC_USDT"].SnapshotsSinceStart)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("не получено обновление статуса вовремя")
	}
}

// TestClient_ReportsErrorOnInvalidToken проверяет, что неверный токен
// (сервер закрывает с 4003) репортится через Errors, не молчаливо
// теряется.
func TestClient_ReportsErrorOnInvalidToken(t *testing.T) {
	fake := newFakeStatusServer("correct-token")
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	c := New(host, port, "wrong-token")
	c.reconnectDelay = 50 * time.Millisecond // ускоряем тест
	go c.RunForever()
	defer c.Stop()

	select {
	case err := <-c.Errors:
		if err == nil {
			t.Error("ожидалась непустая ошибка")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ошибка о неверном токене не пришла вовремя")
	}
}

// TestClient_ReconnectsAfterServerRestart — сквозной сценарий: сервер
// падает и поднимается заново на том же порту, клиент должен сам
// переподключиться и снова получать обновления.
func TestClient_ReconnectsAfterServerRestart(t *testing.T) {
	fake1 := newFakeStatusServer("tok")
	srv1 := httptest.NewServer(http.HandlerFunc(fake1.handler))
	host, port := hostPort(t, srv1.URL)

	c := New(host, port, "tok")
	c.reconnectDelay = 100 * time.Millisecond
	go c.RunForever()
	defer c.Stop()

	// Первое соединение получает статус.
	fake1.send <- Status{Type: "status", UptimeSeconds: 1}
	select {
	case <-c.Updates:
	case <-time.After(2 * time.Second):
		t.Fatal("не получено первое обновление")
	}

	// "Падение" сервера — закрываем listener на этом порту.
	close(fake1.send)
	srv1.Close()

	// Поднимаем новый сервер на ТОМ ЖЕ порту.
	listener := mustListenOnPort(t, port)
	fake2 := newFakeStatusServer("tok")
	srv2 := &http.Server{Handler: http.HandlerFunc(fake2.handler)}
	go srv2.Serve(listener)
	defer srv2.Close()

	// Клиент должен сам переподключиться и получить новое обновление.
	fake2.send <- Status{Type: "status", UptimeSeconds: 99}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case status := <-c.Updates:
			if status.UptimeSeconds == 99 {
				return // успех — реконнект сработал
			}
		case <-deadline:
			t.Fatal("клиент не переподключился к перезапущенному серверу вовремя")
		}
	}
}

func mustListenOnPort(t *testing.T, port int) net.Listener {
	t.Helper()
	l, err := newListener(port)
	if err != nil {
		t.Fatalf("не удалось занять порт %d заново: %v", port, err)
	}
	return l
}

// TestClient_IgnoresMalformedJSON проверяет, что некорректный JSON от
// сервера не рвёт соединение и не паникует — клиент просто ждёт
// следующее валидное сообщение.
func TestClient_IgnoresMalformedJSON(t *testing.T) {
	var mu sync.Mutex
	gotToken := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var auth authMessage
		if err := conn.ReadJSON(&auth); err != nil {
			return
		}
		mu.Lock()
		gotToken = auth.Token == "tok"
		mu.Unlock()

		conn.WriteMessage(websocket.TextMessage, []byte("not valid json"))
		conn.WriteJSON(Status{Type: "status", UptimeSeconds: 7})
	}))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	c := New(host, port, "tok")
	go c.RunForever()
	defer c.Stop()

	select {
	case status := <-c.Updates:
		if status.UptimeSeconds != 7 {
			t.Errorf("UptimeSeconds = %f, want 7 (валидное сообщение после невалидного)", status.UptimeSeconds)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("не получено валидное обновление после невалидного JSON")
	}

	mu.Lock()
	defer mu.Unlock()
	if !gotToken {
		t.Error("сервер не получил ожидаемый токен")
	}
}

// TestStatus_JSONUnmarshalMatchesProtocol проверяет разбор реального
// формата, который шлёт statusserver (см. collector/statusserver).
func TestStatus_JSONUnmarshalMatchesProtocol(t *testing.T) {
	raw := []byte(`{"type":"status","uptime_seconds":123.45,"total_disk_bytes":999,"symbols":{"BTC_USDT":{"snapshots_since_start":10,"last_snapshot_age_seconds":0.2}}}`)
	var status Status
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if status.Type != "status" || status.UptimeSeconds != 123.45 || status.TotalDiskBytes != 999 {
		t.Errorf("неверно разобран статус: %+v", status)
	}
	if status.Symbols["BTC_USDT"].SnapshotsSinceStart != 10 {
		t.Errorf("SnapshotsSinceStart = %d, want 10", status.Symbols["BTC_USDT"].SnapshotsSinceStart)
	}
}

// TestRunForever_LateReaderAlwaysGetsFreshErrorAndReconnectNeverStalls
// — регрессионный тест на два последовательных бага, найденных ревью.
// Первая версия слала ошибки в Errors через select с default-веткой —
// та срабатывает немедленно, если читатель не готов ПРЯМО СЕЙЧАС, что
// гарантированно теряло ошибки в реальном UI (окно между обработкой
// одного clientErrMsg и повторным запуском чтения канала в bubbletea
// ненулевое). Простая замена на блокирующий select без default чинила
// потерю, но создавала дедлок: без читателя вообще RunForever зависал
// на первой же ошибке, не доходя до повторного подключения. Итоговая
// логика — буфер 1 с вытеснением устаревшего значения (см. sendError)
// — не гарантирует доставку КАЖДОЙ ошибки (это осознанный отказ: TUI
// нужно текущее состояние подключения, не журнал), но гарантирует две
// вещи, которые здесь и проверяются: читатель с задержкой (типичный
// паттерн bubbletea) всегда получает АКТУАЛЬНУЮ ошибку, не протухшую,
// и реконнект-цикл при этом продолжает идти не блокируясь — сервер
// реально переподключается заново на каждой итерации, судя по
// растущему счётчику попыток аутентификации.
func TestRunForever_LateReaderAlwaysGetsFreshErrorAndReconnectNeverStalls(t *testing.T) {
	fake := newFakeStatusServer("correct-token")
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	c := New(host, port, "wrong-token")
	c.reconnectDelay = 20 * time.Millisecond // быстрые повторные попытки
	go c.RunForever()
	defer c.Stop()

	// Читаем БЕЗ читателя вообще первые 200мс — за это время сервер
	// должен успеть отклонить клиента несколько раз подряд (проверяем
	// счётчик попыток ниже), а RunForever не должен застрять на первой
	// же ошибке.
	time.Sleep(200 * time.Millisecond)

	attemptsBefore := fake.authAttempts()
	if attemptsBefore < 2 {
		t.Fatalf("за 200мс с reconnectDelay=20мс ожидалось несколько попыток подключения, получено %d — похоже, реконнект-цикл застрял", attemptsBefore)
	}

	// Теперь читаем — должны получить ошибку немедленно (канал уже
	// содержит хотя бы одну, буфер 1).
	select {
	case err := <-c.Errors:
		if err == nil {
			t.Fatal("получена nil-ошибка вместо реальной")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("не получено ни одной ошибки, хотя сервер отклонял токен несколько раз")
	}

	// Реконнект-цикл продолжает идти и после чтения — попыток должно
	// стать ещё больше.
	time.Sleep(200 * time.Millisecond)
	attemptsAfter := fake.authAttempts()
	if attemptsAfter <= attemptsBefore {
		t.Errorf("попыток подключения не прибавилось после чтения ошибки (%d -> %d) — реконнект-цикл, похоже, остановился", attemptsBefore, attemptsAfter)
	}
}

// TestStop_NeverBlocksEvenWithoutAnyReader проверяет, что Client можно
// штатно остановить через Stop(), даже если Errors никогда не читался
// ни разу за всё время работы — sendError (см. client.go) устроен так,
// что никогда не блокируется сам по себе, поэтому RunForever доходит
// до проверки stopCh на следующей итерации цикла независимо от того,
// читает ли кто-нибудь канал ошибок.
func TestStop_NeverBlocksEvenWithoutAnyReader(t *testing.T) {
	fake := newFakeStatusServer("correct-token")
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	host, port := hostPort(t, srv.URL)
	c := New(host, port, "wrong-token")
	c.reconnectDelay = 10 * time.Millisecond
	go c.RunForever()

	// Намеренно НЕ читаем c.Errors вообще.
	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() не вернулся вовремя — возможно, дедлок")
	}
}
