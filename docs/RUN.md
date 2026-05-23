# Локальный запуск

## Быстрый старт (docker-compose)

```bash
task compose:up      # docker compose up -d --build
task compose:logs    # tail сервиса в follow
task compose:down    # остановить
task compose:nuke    # снести вместе с volume БД
```

Что произойдёт:

1. **postgres** поднимается с пустой БД (volume `sso-pg-data`).
2. **sso** ждёт `healthy` postgres, стартует, прогоняет goose-миграции прямо
   в `storage.New()`, поднимает gRPC на `0.0.0.0:44044`.
3. **sso-seed** — одноразовый sidecar: ждёт появления таблицы `apps` и вставляет
   demo-приложение `(id=1, secret='replace-me-with-strong-secret')`. Без него
   `Login`/`Validate` ничего не сделают (`app_id` обязателен).

Postgres снаружи **не выставлен** — порт 5432 у тебя часто занят чужими
контейнерами. Если нужен прямой доступ:
```bash
docker exec -it sso-postgres psql -U sso -d sso
```
или раскомментируй `ports: ["5433:5432"]` в `docker-compose.yml`.

## Проверка через grpcurl

```bash
# поставить grpcurl, если ещё нет
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# список сервисов
grpcurl -plaintext localhost:44044 list

# регистрация
grpcurl -plaintext -d '{"email":"alice@example.com","password":"longenough"}' \
  localhost:44044 sso.v1.AuthService/Register

# логин
grpcurl -plaintext -d '{"email":"alice@example.com","password":"longenough","app_id":1}' \
  localhost:44044 sso.v1.AuthService/Login
```

## Запуск без docker (для разработки)

Если правишь Go-код и не хочешь каждый раз пересобирать образ — подними
только postgres из compose, а сервис гоняй локально через `go run`.

```bash
docker compose up -d postgres
# на хосте postgres не выставлен — добавь ports или используй docker exec для psql

# открой проброс или поправь DSN под свой инстанс
SSO_DB_DSN="postgres://sso:sso@localhost:5432/sso?sslmode=disable" \
  go run ./cmd/sso

# засей app один раз
docker exec sso-postgres psql -U sso -d sso -c \
  "INSERT INTO apps (id, name, secret) VALUES (1, 'demo', 'dev-secret') ON CONFLICT DO NOTHING"
```

## Переменные окружения

Все YAML-поля можно переопределить env (env приоритетнее):

| Env | Назначение | Дефолт |
|---|---|---|
| `SSO_ENV` | `local` / `prod` (включает JSON-логи) | `local` |
| `SSO_GRPC_PORT` | порт gRPC | `44044` |
| `SSO_GRPC_TIMEOUT` | таймаут запроса | `10s` |
| `SSO_DB_DSN` | DSN PostgreSQL | (обязателен, если нет YAML) |
| `SSO_ACCESS_TTL` | TTL access-токена | `15m` |
| `SSO_REFRESH_TTL` | TTL refresh-токена | `720h` |
| `CONFIG_PATH` | путь к YAML | `./config/local.yaml` |
