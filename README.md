# Go DevTrace

Go DevTrace — лёгкая надстройка над стандартным пакетом `log`, которая добавляет удобно читаемый стек вызовов к каждому сообщению. Подключается одной строчкой, не требует переписывать существующие логи и хорошо работает в паре с контекстом.

## Основные возможности

- 📞 **Стек из коробки** — любой `log.Printf/Print/Fatal/Panic` автоматически печатает цепочку вызовов, подсвечивая место логирования.
- 🔧 **Минимальная интеграция** — достаточно один раз вызвать `RedirectStandardLogger()`, весь остальной код остаётся прежним.
- 🧭 **Гибкая настройка** — управляйте глубиной стека, количеством строк кода вокруг места вызова, шаблоном файлов вашего приложения.
- ⏱️ **Вспомогательные утилиты** — при необходимости доступны `TraceFunc`, измерение времени и бенчмарки.
- ✅ **Тесты** — behavior закреплён в `stack_logger_test.go`.

## Быстрый старт

```go
import (
    "log"

    devtrace "github.com/hackathon/gotrace"
)

func init() {
    devtrace.SetConfig(devtrace.DevTraceConfig{
        Enabled:     true,
        StackLimit:  5,
        ShowSnippet: 2,
        AppPattern:  "github.com/your-org/your-service",
        DebugLevel:  1,
    })

    devtrace.InstallStackLogger(&devtrace.StackLoggerOptions{
        Prefix:     "📞 CALL STACK",
        Skip:       2,
        Limit:      5,
        AppPattern: "github.com/your-org/your-service",
        Ascending:  true,
    })

    devtrace.RedirectStandardLogger()
}

func main() {
    log.Printf("hello world")
}
```

### Пример вывода

```
[DEVTRACE-INFO] 📞 CALL STACK
  Route: main
  1. main.go:18 → main()
        16 func main() {
        17     // …
      > 18     log.Printf("hello world")

Message Log: hello world
```

## Дополнительные API

- `TraceFunc` / `TraceWithOptions` — обёртка функций в трейс-контекст (полезно для измерения времени и получения стека без стандартного логгера).
- `TimeFunc`, `TimeFuncWithResult`, `BenchmarkFunc` — быстрая диагностика производительности.

## Пример

Проект `example/` содержит живую демонстрацию:

```bash
cd example
GOCACHE=$(pwd)/../.gocache go run .
```

## Тесты

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```

## Лицензия

Проект распространяется по лицензии MIT.
