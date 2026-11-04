# TripGo

Учебный сервис для создания, получения и завершения поездок. Данные хранятся
в PostgreSQL, HTTP-ручки описаны в OpenAPI.

## Архитектура

Зависимости идут внутрь: `http` преобразует запросы/ответы и вызывает usecase;
usecase зависит от домена, моделей и узких интерфейсов; repository реализует
интерфейсы хранения поверх PostgreSQL. `infra` содержит пул базы, транзакции,
логи и OpenTelemetry. `domain/trip` хранит правила перехода статусов и доменные
ошибки, не импортируя транспорт или инфраструктуру. `model` содержит структуры,
которые передаются между слоями.

Сгенерированный OpenAPI-роутер направляет запрос в HTTP-обработчик. Он
валидирует JSON и переводит входной формат в usecase input. Создание поездки и
начальной записи истории, а также завершение поездки и запись истории выполняются
в транзакциях. PostgreSQL ограничивает поездки водителя уникальным частичным
индексом.

При создании поездки сервис в одной транзакции сохраняет поездку и начальную
запись истории статусов. При завершении блокируется строка поездки, меняется
статус и добавляется запись в историю. Уникальный индекс в БД не позволяет
водителю иметь две активные поездки одновременно.

## Наблюдаемость

Логи выводятся через `slog` в JSON, middleware добавляет данные запроса и
корреляцию `trace_id`/`span_id`. Middleware оборачиваются в порядке Recover,
Tracing, Logging, Metrics. OpenTelemetry экспортирует HTTP и PostgreSQL трейсы,
а также метрики по OTLP. Если коллектор недоступен, экспортёр повторяет
отправку асинхронно, обработка HTTP-запросов не зависит от его доступности.

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

Команды:

```sh
make generate
make migrate
make migrate-down
make test
make test-integration
make lint
make build
```

`make test-integration` читает `DATABASE_URL` из `.env`

Для OTLP задайте `OTEL_EXPORTER_OTLP_ENDPOINT` и
`OTEL_EXPORTER_OTLP_PROTOCOL` (`grpc` или `http/protobuf`).

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
