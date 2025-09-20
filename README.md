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

    devtrace "github.com/skulidropek/gotrace"
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

## Расширенная интеграция

Ниже пример, как вынести настройку в отдельный пакет и управлять поведением через переменные окружения. Такой пакет можно подключать blank-import'ом.

```go
// pkg/logging/init.go
package logging

import (
    "log"
    "os"
    "strconv"
    "strings"

    devtrace "github.com/skulidropek/gotrace"
)

func init() {
    cfg := devtrace.DevTraceConfig{
        Enabled:     envBool("DEVTRACE_ENABLED", true),
        StackLimit:  envInt("DEVTRACE_STACK_LIMIT", 6),
        ShowSnippet: envInt("DEVTRACE_SHOW_SNIPPET", 2),
        AppPattern:  getenv("DEVTRACE_APP_PATTERN", "github.com/your-org/your-app"),
        DebugLevel:  envInt("DEVTRACE_DEBUG_LEVEL", 1),
    }

    devtrace.SetConfig(cfg)
    devtrace.InstallStackLogger(&devtrace.StackLoggerOptions{
        Prefix:      "📞 CALL STACK",
        Skip:        envInt("DEVTRACE_STACK_SKIP", 2),
        Limit:       cfg.StackLimit,
        ShowSnippet: cfg.ShowSnippet,
        OnlyApp:     envBool("DEVTRACE_ONLY_APP", false),
        PreferApp:   envBool("DEVTRACE_PREFER_APP", true),
        AppPattern:  cfg.AppPattern,
        Ascending:   envBool("DEVTRACE_ASCENDING", true),
    })

    devtrace.RedirectStandardLogger()

    log.Printf("logging initialized")
}

func getenv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func envInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}

func envBool(key string, def bool) bool {
    if v := os.Getenv(key); v != "" {
        switch strings.ToLower(v) {
        case "1", "true", "t", "yes", "y":
            return true
        case "0", "false", "f", "no", "n":
            return false
        }
    }
    return def
}
```

Пакет подключается blank-import'ом:

```go
import (
    _ "github.com/your-org/your-app/pkg/logging"
)
```

### Переменные окружения

| Переменная               | Назначение                           | Значение по умолчанию |
|--------------------------|--------------------------------------|------------------------|
| `DEVTRACE_ENABLED`       | Включить/выключить DevTrace          | `true`                 |
| `DEVTRACE_STACK_LIMIT`   | Количество кадров в стеке            | `6`                    |
| `DEVTRACE_SHOW_SNIPPET`  | Строки кода вокруг мест вызова       | `2`                    |
| `DEVTRACE_STACK_SKIP`    | Сколько внутренних кадров пропустить | `2`                    |
| `DEVTRACE_ONLY_APP`      | Показывать только файлы приложения   | `false`                |
| `DEVTRACE_PREFER_APP`    | Отдавать приоритет кадрам приложения | `true`                 |
| `DEVTRACE_ASCENDING`     | `true` — root→call-site              | `true`                 |
| `DEVTRACE_APP_PATTERN`   | Шаблон путей приложения              | `github.com/...`       |
| `DEVTRACE_DEBUG_LEVEL`   | 0..2, подробность служебных логов    | `1`                    |
| `LOG_FILE`               | Путь к файлу (если нужен)            | —                      |
| `LOG_CONSOLE`            | Дублировать в stderr (`true/false`)  | `true`                 |
| `LOG_LEVEL`              | Уровень вашего логгера               | `info`                 |

Эта схема подходит, если нужен сценарий «подключил и забыл»: стандартный `log.*` и любые дополнительные логгеры, которые вы инициализируете внутри `init()`, сразу получают стек вызовов без переписывания бизнес-кода.

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
