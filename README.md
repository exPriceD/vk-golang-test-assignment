# Тестовое задание на позицию Golang-разработчик в VK

Реализовать  примитивный worker-pool с возможностью динамически добавлять и удалять воркеры. 
Входные данные (строки) поступают в канал, воркеры их обрабатывают 
(например, выводят на экран номер воркера и сами данные). Задание на базовые знания каналов и горутин.

# Решение
Проект реализует пул воркеров (WorkerPool) с поддержкой асинхронной обработки
строковых задач, поступающих в канал. Решение построено на принципах 
чистой архитектуры, SOLID и лучших практик Go, обеспечивая модульность, 
тестируемость и масштабируемость.

**Основной акцент сделан на:**
- **Модульность:** Разделение на слои (интерфейсы, логика, ошибки) 
и использование интерфейсов для инверсии зависимостей.

- **Конкурентность:** Безопасное управление горутинами через каналы, 
контексты и атомарные операции.

- **Тестируемость:** Полное покрытие модульными тестами с использованием мок-объектов.

- **Логирование:** Детализированные логи через 
zap для отладки и мониторинга.

### Основные возможности:
- Создание пула с настраиваемым числом воркеров и размером буфера задач.
- Динамическое добавление (`AddWorkers`) и удаление (`RemoveWorkers`) воркеров.
- Асинхронная отправка задач (`Submit`) с таймаутом при полном буфере.
- Логирование операций через интерфейс interfaces.Logger (реализация с zap).
- Обработка ошибок через кастомные типы (`errors.ErrInvalidBufferSize`, `errors.ErrTimeout` и др.).
- Graceful завершение работы пула (`Shutdown`) с корректной остановкой воркеров.
- Обработка строковых или других задач (должны реализовывать метод `Process()`) с выводом ID воркера и данных через `DefaultWorker`.


### Примененные практики
- Чистая архитектура: Разделение на слои (интерфейсы в `internal/interfaces`, 
реализация в `workerpool` и `worker`, ошибки в `errors`) обеспечивает независимость компонентов.

- SOLID:
  - **Single Responsibility:** Каждый компонент (`WorkerPool`, `DefaultWorker`, `Task`) выполняет одну задачу.
  - **Open/Closed:** Интерфейсы `Worker` и `Task` позволяют расширять функциональность без изменения кода.
  - **Dependency Inversion:** `WorkerPool` зависит от абстракций (`interfaces.Worker`, `interfaces.Logger`), а не от реализаций.
- **Идемпотентность:** Повторные вызовы `Shutdown` безопасны благодаря атомарному флагу `closed`.
- **Конкурентная безопасность:** Использование `sync.WaitGroup`, `atomic.Int32` и каналов для синхронизации.

# Архитектура
WorkerPool — центральный компонент, управляющий пулом воркеров через интерфейсы 
`Worker`, `Task` и `Logger`. 
Архитектура построена на принципах модульности и инкапсуляции, что упрощает тестирование и расширение.

### Ключевые компоненты
- **WorkerPool**: Управляет пулом через каналы (`tasks`) и контекст (`ctx`),
  синхронизирует воркеры с помощью `sync.WaitGroup` и `atomic.Int32`.

- **Worker**: Реализует метод `Run`, обрабатывающий задачи из канала или завершающийся по контексту.

- **Task**: Определяет задачу с методом `Process` для выполнения полезной работы.

- **Logger**: Логирует события (`"Воркер зарегистрирован"`, `"Таймаут отправки задачи: буфер полон"`)
  с уровнями `Debug`, `Info`, `Warn`, `Error`.

### Структура проекта
```
├───cmd
│   ├───main.go
├───config
│   ├───config.yaml
│   └───config_test.go
├───internal
│   ├───config
│   │   ├───config.go
│   │   └───config_test.go
│   ├───constants
│   │   └───constants.go
│   ├───errors
│   │   └───errors.go
│   ├───interfaces
│   │   ├───logger.go
│   │   └───task.go
│   ├───logger
│   │   └───logger.go
│   ├───task
│   │   ├───wrapper.go
│   │   └───task.go
│   ├───worker
│   │   ├───worker.go
│   │   └───worker_test.go
│   └───workerpool
│       ├───workerpool.go
│       └───workerpool_test.go
├───Makefile
└───README.md
```

# Установка и запуск
### Требования:
- Go 1.21 или выше
- Git
- Модули: `github.com/stretchr/testify`, `go.uber.org/zap`

### Установка:
1. Клонируйте репозиторий:
```bash
git clone https://github.com/exPriceD/vk-golang-test-assignment.git
cd vk-golang-test-assignment
```

2. Установите зависимости:
```bash
go mod tidy
```

### Вариант 1: Запуск через Makefile
Makefile автоматизирует сборку, запуск и тестирование приложения.
1. Убедитесь, что файл `config/config.yaml` настроен.
2. Выполните:
```bash
make
```

Это установит зависимости, соберет бинарник и запустит его 
с конфигурацией config/config.yaml.

**Другие полезные команды:**
```bash
make build  # Собрать бинарник
make run    # Запустить приложение
make test   # Запуск тестов
make clean  # Очистить бинарник и кэш
make help   # Список команд
```

### Вариант 2: Ручной запуск
1. Соберите приложение:
```bash
go build -o vk-worker-pool ./cmd/main.go
```

2. Запустите, указав конфигурацию (опционально, по стандарту ищет в папке `config`):
```bash
./vk-worker-pool -config=config/config.yaml
```

# Тестирование
Проект покрыт модульными тестами для пакетов config, workerpool и worker.

## Запуск тестов
Через Makefile:
```bash
make test
```

Или вручную:
```bash
go test -v ./internal/...
```

Покрытие:
```bash
go test -cover ./internal/...
```
```bash
ok      vk-worker-pool/internal/config  0.202s  coverage: 93.1% of statements
ok      vk-worker-pool/internal/worker  0.207s  coverage: 100.0% of statements
ok      vk-worker-pool/internal/workerpool      0.186s  coverage: 85.2% of statements
```

# Методы WorkerPool
### NewWorkerPool
```go
func NewWorkerPool(initialWorkers int, taskBufferSize int, worker interfaces.Worker, log interfaces.Logger, poolTimeout time.Duration) (*WorkerPool, error)
```
**Аргументы:**
- `initialWorkers int`: Начальное число воркеров (≥ 0).
- `taskBufferSize int`: Размер буфера задач (≥ 1).
- `worker interfaces.Worker`: Реализация воркера.
- `log interfaces.Logger`: Логгер для событий.
- `poolTimeout time.Duration`: Таймаут для операций (например, отправки задач).

**Возвращает:** Указатель на `WorkerPool` или ошибку (`ErrInvalidBufferSize`, `ErrInvalidWorkerCount`). 

**Описание:** Создает пул с заданным числом воркеров и буфером задач. Запускает воркеры через `AddWorkers` и логирует создание ("Создан новый пул воркеров").

---

### AddWorkers
```go
func (wp *WorkerPool) AddWorkers(numWorkers int)
```
**Аргументы:**
- `numWorkers int`: Число добавляемых воркеров (≥ 1).

**Возвращает:** -

**Описание:** Добавляет воркеры, увеличивая `workerCount` атомарно. Каждый воркер запускается в отдельной горутине, выполняя `worker.Run`. Логирует успех (`"Добавлены новые воркеры"`) или ошибки (`"Попытка добавить неположительное количество воркеров"`).

---

### RemoveWorkers
```go
func (wp *WorkerPool) RemoveWorkers(numWorkers int)
```
**Аргументы:**
- `numWorkers int`: Число удаляемых воркеров (≥ 1).

**Возвращает:** -

**Описание:** Останавливает воркеры, отправляя `nil`-задачи в канал `tasks` и уменьшая `workerCount`. Логирует успех (`"Остановлены воркеры"`) или ошибки (`"Таймаут отправки сигнальной задачи: буфер полон"`).

---

### RemoveWorkers
```go
func (wp *WorkerPool) Submit(task interfaces.Task) error
```
**Аргументы:**
- `task interfaces.Task`: Задача для обработки.

**Возвращает:** Ошибку (`ErrNilTask`, `ErrPoolStopped`, `ErrTimeout`) или `nil`.

**Описание:** Отправляет задачу в канал `tasks`. Если буфер полон, ждет `poolTimeout`. Логирует успех (`"Задача отправлена в пул"`) или ошибки (`"Таймаут отправки задачи: буфер полон"`).

---

### Shutdown
```go
func (wp *WorkerPool) Shutdown()
```
**Аргументы:** - 

**Возвращает:** -

**Описание:** Завершает пул, отменяя контекст, ожидая завершения воркеров и закрывая канал `tasks`. Логирует этапы (`"Инициирована остановка пула воркеров", "Пул воркеров успешно остановлен"`) и предупреждения (`"Таймаут ожидания завершения воркеров"`).