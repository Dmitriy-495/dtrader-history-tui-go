package statusclient

import (
	"fmt"
	"net"
)

// newListener захватывает TCP-порт заново — используется только в
// тестах, чтобы симулировать перезапуск сервера на том же порту
// (см. TestClient_ReconnectsAfterServerRestart). Между Close()
// предыдущего listener'а и вызовом этой функции ОС может ненадолго
// удерживать порт в TIME_WAIT — если это станет проблемой на CI,
// стоит перейти на net.Listen с SO_REUSEADDR или ретраить с паузой.
func newListener(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}
