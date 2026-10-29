# TripGo

Учебный сервис для создания, получения и завершения поездок. Данные хранятся
в PostgreSQL, HTTP-ручки описаны в OpenAPI.

## Как работает

Запрос сначала попадает в сгенерированный роутер, затем в HTTP-обработчик.
Обработчик проверяет входные данные и вызывает service. Service выполняет
логику поездки, а repository собирает SQL через squirrel и выполняет его через
pgx.

При создании поездки сервис в одной транзакции сохраняет поездку и начальную
запись истории статусов. При завершении блокируется строка поездки, меняется
статус и добавляется запись в историю. Уникальный индекс в БД не позволяет
водителю иметь две активные поездки одновременно.

Реализованы ручки:

- `POST /api/v1/trips` — создать поездку;
- `GET /api/v1/trips/{tripId}` — получить поездку;
- `POST /api/v1/trips/{tripId}/finish` — завершить поездку;
- `GET /health` — проверить, что сервис работает;
- `GET /ready` — проверить доступность PostgreSQL.

## Запуск

Для локального окружения курса:

```sh
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl connect` создаёт `.env` с настройками подключения. Если запускаю
свою PostgreSQL, копирую `.env.example` в `.env` и меняю `DATABASE_URL`.
Обязательные настройки проверяются при старте.

Полезные команды:

```sh
make generate      # сгенерировать API-код из OpenAPI
make migrate       # применить миграции
make migrate-down  # откатить миграции
make test          # запустить проверки с race detector
make build         # собрать бинарный файл
```

При остановке по `Ctrl+C` или сигналу `SIGTERM` сервер ждёт текущие запросы в
пределах `SHUTDOWN_TIMEOUT`.

## Docker (дополнительное задание)

Сборка многостадийная: Go собирает статический бинарный файл, а в финальный
образ `distroless` копируется только он.

Поднять окружение и применить миграции:

```sh
docker compose up -d postgres
go tool goose -dir migrations postgres \
  'postgres://tripgo:tripgo@localhost:5432/tripgo?sslmode=disable' up
docker compose up -d --build
curl http://localhost:8080/ready
```

Остановить контейнеры:

```sh
docker compose down
```

Размер собранного итогового образа `tripgo-trip-service` — около 4,4 MB
