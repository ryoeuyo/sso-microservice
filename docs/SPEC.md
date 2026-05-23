# SSO Microservice — Техническое задание

## 1. Назначение

gRPC-сервис единого входа (Single Sign-On) для множества клиентских приложений.
Отвечает за регистрацию, аутентификацию пользователей, выдачу и валидацию JWT-токенов,
а также за проверку прав доступа (роли) в рамках конкретного приложения (`app_id`).

Сервис не имеет UI — это backend-компонент, к которому обращаются другие сервисы по gRPC.

---

## 2. Функциональные требования

### 2.1 Аутентификация
- **Register** — регистрация пользователя по `email` + `password`. Пароль хешируется (bcrypt/argon2id).
- **Login** — вход по `email` + `password` + `app_id`. Возвращает пару `access_token` + `refresh_token`.
- **Refresh** — обновление пары токенов по валидному `refresh_token` с ротацией (старый refresh инвалидируется).
- **Logout** — инвалидация refresh-токена (выход с текущего устройства).
- **Validate** — проверка `access_token`, возвращает `user_id`, `app_id`, `roles`, `exp`.

### 2.2 Управление пользователями
- **GetUser** — получить пользователя по `id`.
- **DeleteUser** — soft delete (флаг `deleted_at`).

### 2.3 Авторизация
- **IsAdmin** — проверка, является ли пользователь администратором в рамках `app_id`.
- Роли/permissions выдаются в контексте приложения (`user_roles` связаны с `app_id`).

### 2.4 Управление приложениями
- В БД хранится таблица `apps` с уникальным `secret` для подписи JWT каждого приложения.
- На MVP админка приложений не нужна — записи создаются миграциями/seed.

---

## 3. Нефункциональные требования

- **Язык**: Go 1.26+
- **Транспорт**: gRPC (protobuf), reflection включён для отладки
- **БД**: PostgreSQL 16+
- **Конфигурация**: YAML + env-переменные (env приоритетнее)
- **Логирование**: structured logging через `log/slog`
- **Graceful shutdown** по `SIGINT`/`SIGTERM`
- **Миграции**: применяются автоматически при старте сервиса (внутри инициализации DB-клиента)
- **Тесты**: unit на доменный слой и usecase, integration на репозитории (через testcontainers)

---

## 4. Технологический стек

### 4.1 Язык и сборка
| Технология | Назначение |
|---|---|
| **Go 1.26+** | основной язык |
| **Go modules** | управление зависимостями |
| **Taskfile / Makefile** | команды разработки (gen, lint, test, run) |

### 4.2 gRPC и Protobuf
| Библиотека | Назначение |
|---|---|
| **buf** ([bufbuild/buf](https://github.com/bufbuild/buf)) | управление `.proto` файлами: lint, breaking-check, кодогенерация |
| **google.golang.org/grpc** | gRPC-сервер |
| **google.golang.org/protobuf** | runtime для сгенерированного кода |
| **protoc-gen-go**, **protoc-gen-go-grpc** | плагины кодогенерации (через `buf generate`) |
| **grpc-ecosystem/go-grpc-middleware** | интерсепторы: recovery, logging, validation |
| **protovalidate-go** | валидация входящих запросов через `buf.validate` правила в `.proto` |

### 4.3 База данных
| Библиотека | Назначение |
|---|---|
| **PostgreSQL** | основное хранилище |
| **jackc/pgx/v5** + **pgxpool** | драйвер и пул соединений |
| **pressly/goose/v3** | миграции; применяются программно при инициализации `db`-клиента |

> Миграции лежат в `migrations/` (формат goose), эмбедятся через `//go:embed`
> и выполняются в `storage.New(...)` сразу после открытия пула.

### 4.4 Безопасность и токены
| Библиотека | Назначение |
|---|---|
| **golang-jwt/jwt/v5** | выпуск и парсинг JWT (access/refresh) |
| **golang.org/x/crypto/bcrypt** | хеширование паролей (можно заменить на argon2id из `x/crypto/argon2`) |

### 4.5 Конфигурация и утилиты
| Библиотека | Назначение |
|---|---|
| **ilyakaznacheev/cleanenv** | парсинг YAML + env в структуру конфига |
| **google/uuid** | генерация идентификаторов |
| **log/slog** (stdlib) | structured logging |

### 4.6 Тестирование
| Библиотека | Назначение |
|---|---|
| **stretchr/testify** | assert/require/mock |
| **testcontainers-go** | поднятие Postgres в Docker для интеграционных тестов |
| **mockery** или **gomock** | генерация моков для интерфейсов |

### 4.7 Качество кода
| Инструмент | Назначение |
|---|---|
| **golangci-lint** | агрегирующий линтер |
| **buf lint** | линт `.proto` |
| **buf breaking** | контроль обратной совместимости API |

---

## 5. Архитектура (DDD)

Сервис разделён на слои. Зависимости направлены **внутрь**: `adapters → app → domain`.
Domain ничего не знает про gRPC, Postgres, JWT.

```
sso-microservice/
├── api/
│   └── proto/
│       └── sso/v1/
│           ├── sso.proto
│           └── auth.proto
├── buf.yaml
├── buf.gen.yaml
├── cmd/
│   └── sso/
│       └── main.go              # composition root
├── config/
│   ├── local.yaml
│   └── prod.yaml
├── internal/
│   ├── domain/                  # ядро: сущности, value objects, ошибки, интерфейсы репозиториев
│   │   ├── user/
│   │   │   ├── user.go
│   │   │   └── errors.go
│   │   ├── token/
│   │   └── app/
│   ├── app/                     # usecases (application services)
│   │   ├── auth/
│   │   │   ├── auth.go          # Register, Login, Refresh, Logout, Validate
│   │   │   └── auth_test.go
│   │   └── permissions/
│   │       └── permissions.go   # IsAdmin
│   ├── adapters/
│   │   ├── grpc/                # тонкие хендлеры, мапинг proto ↔ domain
│   │   │   ├── auth_handler.go
│   │   │   └── server.go
│   │   ├── storage/
│   │   │   └── postgres/
│   │   │       ├── postgres.go  # pgxpool + goose миграции
│   │   │       ├── user_repo.go
│   │   │       └── app_repo.go
│   │   └── jwt/
│   │       └── issuer.go        # выпуск/парсинг токенов
│   └── config/
│       └── config.go
├── migrations/                  # .sql файлы для goose, эмбедятся в бинарь
│   ├── 0001_init.sql
│   └── 0002_apps.sql
├── gen/                         # сгенерированный код из proto (в .gitignore при желании)
│   └── sso/v1/
├── docs/
│   └── SPEC.md
├── go.mod
└── go.sum
```

### 5.1 Слои

- **domain** — сущности (`User`, `App`, `Token`), value objects (`Email`, `PasswordHash`),
  доменные ошибки (`ErrUserNotFound`, `ErrInvalidCredentials`), интерфейсы репозиториев.
  Только Go-код, без сторонних зависимостей (кроме stdlib и `uuid`).
- **app** (usecase) — оркестрация: принимает DTO, дёргает репозитории и доменные методы,
  возвращает результат или доменную ошибку.
- **adapters** — реализация интерфейсов из domain:
  - `grpc` — конвертирует proto-запросы в вызовы usecase и обратно;
  - `storage/postgres` — pgx-репозитории;
  - `jwt` — реализация `TokenIssuer`.
- **cmd/sso/main.go** — composition root: читает конфиг, создаёт зависимости, поднимает grpc-сервер.

---

## 6. Контракты API (черновик)

Пакет: `sso.v1`. Определяются в `api/proto/sso/v1/`.

```
service Auth {
  rpc Register (RegisterRequest) returns (RegisterResponse);
  rpc Login    (LoginRequest)    returns (LoginResponse);
  rpc Refresh  (RefreshRequest)  returns (LoginResponse);
  rpc Logout   (LogoutRequest)   returns (LogoutResponse);
  rpc Validate (ValidateRequest) returns (ValidateResponse);
}

service Permissions {
  rpc IsAdmin (IsAdminRequest) returns (IsAdminResponse);
}
```

Ошибки маппятся из доменных в gRPC коды:
- `ErrUserNotFound` → `NOT_FOUND`
- `ErrInvalidCredentials` → `UNAUTHENTICATED`
- `ErrUserAlreadyExists` → `ALREADY_EXISTS`
- `ErrInvalidArgument` → `INVALID_ARGUMENT`

---

## 7. Схема БД (черновик)

```sql
-- 0001_init.sql
CREATE TABLE users (
    id         UUID PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    pass_hash  BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE apps (
    id     INT PRIMARY KEY,
    name   TEXT NOT NULL,
    secret TEXT NOT NULL
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id  INT  NOT NULL REFERENCES apps(id)  ON DELETE CASCADE,
    role    TEXT NOT NULL, -- 'admin' | 'user' | ...
    PRIMARY KEY (user_id, app_id, role)
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id     INT  NOT NULL REFERENCES apps(id)  ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON refresh_tokens (user_id);
```

---

## 8. Миграции (goose в коде)

Миграции эмбедятся в бинарь и применяются на старте, до запуска gRPC-сервера.

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func New(ctx context.Context, dsn string) (*Storage, error) {
    pool, err := pgxpool.New(ctx, dsn)
    if err != nil { return nil, err }

    goose.SetBaseFS(migrationsFS)
    if err := goose.SetDialect("postgres"); err != nil { return nil, err }

    db := stdlib.OpenDBFromPool(pool)
    if err := goose.UpContext(ctx, db, "migrations"); err != nil {
        return nil, fmt.Errorf("migrate: %w", err)
    }
    return &Storage{pool: pool}, nil
}
```

> Откат миграций — отдельной CLI-командой `cmd/migrate` (если понадобится), не в основном бинаре.

---

## 9. Конфигурация (пример)

```yaml
env: local
grpc:
  port: 44044
  timeout: 10s
db:
  dsn: "postgres://sso:sso@localhost:5432/sso?sslmode=disable"
  max_conns: 10
tokens:
  access_ttl: 15m
  refresh_ttl: 720h   # 30 дней
```

Все поля можно переопределить env-переменными (`SSO_GRPC_PORT`, `SSO_DB_DSN`, ...).

---

## 10. Дорожная карта (MVP → v1)

1. **MVP**
   - `Register`, `Login`, `Validate`, `IsAdmin`
   - JWT (access only), bcrypt, pgxpool, goose
   - Конфиг, slog, graceful shutdown
2. **v0.2**
   - Refresh-токены с ротацией, таблица `refresh_tokens`, `Logout`
   - Интерсепторы: recovery, logging, protovalidate
3. **v0.3**
   - Rate limiting на `Login`/`Register`
   - Интеграционные тесты на testcontainers
4. **v1.0**
   - Метрики Prometheus, OpenTelemetry tracing
   - Audit log
   - 2FA (TOTP) — опционально
