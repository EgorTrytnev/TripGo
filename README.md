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
make generate
make migrate
make migrate-down
make test
make build
```

При остановке по `Ctrl+C` или сигналу `SIGTERM` сервер ждёт текущие запросы в
пределах `SHUTDOWN_TIMEOUT`.

## Docker (дополнительное задание)

Сборка многостадийная: Go собирает статический бинарный файл, а в финальный
образ `distroless` копируется только он.
Сначала запусти окружение и создай `.env`:

```sh
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
```

Собери образ и запусти сервис, передав ему настройки из `.env`:

```sh
docker build -t tripgo-trip-service .
```

Перед запуском контейнера открой `.env` и в `DATABASE_URL` замени только хост
`localhost` на `host.docker.internal`. 

После изменения `.env` запусти контейнер:

```sh
docker run --rm --name trip-service -p 8080:8080 --env-file .env tripgo-trip-service
```
Миграции по-прежнему запускаются с хоста командой `make migrate`.

Остановить сервис — `Ctrl+C`. Размер собранного итогового образа
`tripgo-trip-service` — около 4,4 MB.
