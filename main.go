// dtrader-history-tui — независимое приложение для мониторинга
// статуса сборщика (dtrader-history/collector) с любой локальной
// машины по сети. Не требует ничего от collector кроме сетевого
// доступа к его status-серверу и, опционально, к sysagent (см.
// internal/ui/sysclient.go) для графиков CPU/RAM/Disk в rightbar.
//
// Использование:
//
//	./tui-client -host <IP-VPS-сборщика> -symbol BTC_USDT -symbol SOL_USDT -symbol AVAX_USDT
//
// Токен читается из .env (STATUS_TOKEN) — тот же файл и тот же токен,
// что использует collector на стороне сборщика; тот же токен
// используется и для sysagent.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	tea "charm.land/bubbletea/v2"

	"github.com/Dmitriy-495/dtrader-history-tui/internal/ui"
	"github.com/Dmitriy-495/dtui-kit/banner"
)

// stringSlice реализует flag.Value для повторяемого флага -symbol
// (аналог action="append" в argparse: --symbol можно указывать
// несколько раз, см. main.py оригинала).
type stringSlice []string

func (s *stringSlice) String() string {
	return fmt.Sprintf("%v", []string(*s))
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var symbols stringSlice
	host := flag.String("host", "", "IP или домен VPS, где крутится collector")
	port := flag.Int("port", 8765, "порт status-сервера сборщика")
	sysPort := flag.Int("sys-port", 8766, "порт sysagent (метрики CPU/RAM/Disk VPS)")
	checkLayout := flag.Bool("check-layout", false, "проверить раскладку (встроенную или из -layout) на диапазоне ширин и выйти, без запуска интерфейса")
	tickerFont := flag.String("ticker-font", "", "ЭКСПЕРИМЕНТ: крупные тикеры в таблице шрифтом dtui-kit/banner (рекомендуется banner3-mini; доступные: см. ошибку при неверном значении); без флага — обычная таблица")
	layoutPath := flag.String("layout", "", "путь к layout.yaml: зоны, описанные в нём, заменяют встроенные (header, footer); без пересборки")
	flag.Var(&symbols, "symbol", "символ для отображения (можно указать несколько раз, должны совпадать с symbols в config.yaml сборщика)")
	flag.Parse()

	// Режим проверки раскладки: ни хост, ни символы, ни токен не нужны.
	if *checkLayout {
		source := "встроенная"
		if *layoutPath != "" {
			if err := ui.LoadLayout(*layoutPath); err != nil {
				return err
			}
			source = *layoutPath
		}
		if !ui.CheckLayout(os.Stdout, source) {
			return fmt.Errorf("раскладка не прошла проверку")
		}
		return nil
	}

	if *host == "" {
		return fmt.Errorf("флаг -host обязателен")
	}
	if len(symbols) == 0 {
		return fmt.Errorf("нужен хотя бы один -symbol")
	}

	// Внешняя раскладка проверяется до запуска интерфейса: опечатка в
	// ключе, типе, шрифте и т.п. — понятная ошибка в консоль, а не
	// испорченный экран.
	if *layoutPath != "" {
		if err := ui.LoadLayout(*layoutPath); err != nil {
			return err
		}
	}

	// Логи — в файл, не в stdout: bubbletea занимает терминал через
	// alt-screen, вывод в stdout поверх этого испортил бы интерфейс
	// (тот же принцип, что и в main.py оригинала: logging.FileHandler).
	logFile, err := os.OpenFile("tui_client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл лога: %w", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	token, err := loadToken()
	if err != nil {
		return err
	}

	if *tickerFont != "" && !banner.IsKnownFont(banner.Font(*tickerFont)) {
		return fmt.Errorf("неизвестный шрифт -ticker-font %q (доступные: %v)", *tickerFont, banner.Fonts())
	}
	model := ui.New(*host, *port, token, []string(symbols), *sysPort).WithTickerFont(banner.Font(*tickerFont))
	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("ошибка выполнения TUI: %w", err)
	}
	return nil
}

// loadToken читает STATUS_TOKEN из .env (в текущей директории, рядом
// с бинарником — тот же принцип, что и Path(__file__).parent / ".env"
// в main.py оригинала, только там относительно исходника, здесь
// относительно рабочей директории запуска) либо из переменной
// окружения напрямую, если .env не найден — godotenv.Load не
// перезаписывает уже установленные переменные окружения, так что оба
// источника мирно сосуществуют.
func loadToken() (string, error) {
	_ = godotenv.Load(".env")
	if exe, err := os.Executable(); err == nil {
		_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	}

	token := os.Getenv("STATUS_TOKEN")
	if token == "" {
		return "", fmt.Errorf(
			"STATUS_TOKEN не задан. Скопируй .env.example в .env и заполни тем же токеном, " +
				"что использует collector на VPS",
		)
	}
	return token, nil
}
