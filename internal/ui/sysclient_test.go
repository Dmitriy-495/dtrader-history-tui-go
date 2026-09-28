package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func sysHostPort(t *testing.T, serverURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("не удалось разобрать URL %q: %v", serverURL, err)
	}
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("не удалось разобрать порт из %q: %v", serverURL, err)
	}
	return host, port
}

func TestSysClient_FetchesAndParsesRealHTTPResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sys" {
			t.Errorf("path = %q, want /sys", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want 'Bearer secret'", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SysStatus{
			CPUPercent:      12.5,
			MemUsedPercent:  40.0,
			DiskUsedPercent: 60.0,
		})
	}))
	defer srv.Close()

	host, port := sysHostPort(t, srv.URL)
	c := NewSysClient(host, port, "secret")
	go c.RunForever()
	defer c.Stop()

	select {
	case status := <-c.Updates:
		if status.CPUPercent != 12.5 {
			t.Errorf("CPUPercent = %f, want 12.5", status.CPUPercent)
		}
		if status.MemUsedPercent != 40.0 {
			t.Errorf("MemUsedPercent = %f, want 40.0", status.MemUsedPercent)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("не получено обновление вовремя")
	}
}

func TestSysClient_ReportsErrorOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	host, port := sysHostPort(t, srv.URL)
	c := NewSysClient(host, port, "wrong")
	go c.RunForever()
	defer c.Stop()

	select {
	case err := <-c.Errors:
		if err == nil {
			t.Error("ожидалась непустая ошибка")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ошибка не пришла вовремя")
	}
}

func TestSysClient_ReportsErrorOnServerDown(t *testing.T) {
	// Реальный TCP-порт, который никто не слушает — connection refused.
	c := NewSysClient("127.0.0.1", 1, "tok") // порт 1 почти гарантированно свободен и закрыт
	go c.RunForever()
	defer c.Stop()

	select {
	case err := <-c.Errors:
		if err == nil {
			t.Error("ожидалась непустая ошибка")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ошибка не пришла вовремя")
	}
}

func TestSysClient_StopUnblocksRunForever(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	host, port := sysHostPort(t, srv.URL)
	c := NewSysClient(host, port, "wrong")
	go c.RunForever()

	// Намеренно не читаем ни Updates, ни Errors.
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() не вернулся вовремя")
	}
}

func TestSysClient_MalformedJSONReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	host, port := sysHostPort(t, srv.URL)
	c := NewSysClient(host, port, "tok")
	go c.RunForever()
	defer c.Stop()

	select {
	case err := <-c.Errors:
		if err == nil || !strings.Contains(err.Error(), "разобрать") {
			t.Errorf("ожидалась ошибка разбора JSON, получено: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ошибка не пришла вовремя")
	}
}

func TestSysStatus_JSONFieldNamesMatchServer(t *testing.T) {
	// Сверка с реальным форматом sysagent/server.SysStatus — если поля
	// разойдутся, JSON просто не распарсится в нужные значения молча
	// (нулевые значения без ошибки), так что явная проверка полезна.
	raw := []byte(`{"cpu_percent":1.5,"mem_total_bytes":100,"mem_used_bytes":50,"mem_used_percent":50.0,"disk_total_bytes":1000,"disk_used_bytes":300,"disk_free_bytes":700,"disk_used_percent":30.0}`)
	var status SysStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if status.CPUPercent != 1.5 || status.MemTotalBytes != 100 || status.DiskFreeBytes != 700 {
		t.Errorf("неверно разобран статус: %+v", status)
	}
}
