# TripGo

Учебный HTTP-сервис поездок на Go. В этой работе реализованы создание,
получение и завершение поездки, хранение в PostgreSQL и журнал переходов
статуса.

## Запуск локально

1. Установить Go из `go.mod`, PostgreSQL и `tripgoctl`.
2. Для первого запуска поднять кластер и окружение курса: `tripgoctl cluster
   start`, `tripgoctl environment start`, `tripgoctl connect`. Последняя команда
   создаст `.env` с параметрами подключения. Если запускаете без `tripgoctl`,
   скопируйте `.env.example` в `.env` и укажите свой `DATABASE_URL`.
3. Применить миграции и запустить сервис:

```sh
make migrate
make run
```

Основные команды:

```sh
make generate      # обновить код из OpenAPI
make migrate       # применить миграции
	make migrate-down  # откатить все миграции
make test          # запустить go test с race detector
make build         # собрать бинарный файл
```

## Переменные окружения

Обязательные переменные указаны в `.env.example`: `HTTP_ADDR`, `LOG_LEVEL`,
`SHUTDOWN_TIMEOUT`, `DATABASE_URL`, `DATABASE_MAX_CONNS`,
`DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`,
`DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT`. Таймауты HTTP-сервера
можно задать через `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`,
`HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`; при отсутствии применяются значения
из примера.

При недоступной БД сервис не запускается. `/health` проверяет только процесс,
`/ready` проверяет соединение с PostgreSQL.

## Реализованные маршруты

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/v1/trips` | создать активную поездку и первую запись истории |
| `GET` | `/api/v1/trips/{tripId}` | получить поездку |
| `POST` | `/api/v1/trips/{tripId}/finish` | завершить поездку и записать переход статуса |
| `GET` | `/health` | liveness |
| `GET` | `/ready` | готовность PostgreSQL |

Ошибки API возвращаются в формате `application/problem+json`.

## Решения

- Для транзакций используется уровень `READ COMMITTED` — стандартный уровень
  PostgreSQL, достаточный здесь вместе с блокировкой строки при завершении и
  уникальным частичным индексом при создании.
- `TxManager.Do` кладёт `pgx.Tx` в контекст. Repository выбирает транзакцию из
  контекста или использует пул, если вызван отдельно.
- Создание поездки и начальная запись истории выполняются внутри одного `Do`.
  Завершение поездки блокирует строку через `SELECT ... FOR UPDATE`, затем
  обновляет статус и пишет историю в той же транзакции.
- Частичный уникальный индекс на `trips(driver_id) WHERE status = 'active'`
  запрещает две активные поездки водителя даже при одновременных запросах.

Идемпотентность и Docker-сборка со звёздочкой не реализованы.
