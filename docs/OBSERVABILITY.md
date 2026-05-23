# Observability

Стек — чистый OpenTelemetry: приложение шлёт OTLP/gRPC в `otel-collector`,
дальше collector раскидывает в три backend'а:

```
sso ─OTLP/gRPC──► otel-collector ─┬─► Tempo        (traces)
                                  ├─► Prometheus   (metrics, scrape)
                                  └─► Loki         (logs, OTLP/HTTP)
                                                  ▲
                                                  │ datasources
                                              Grafana :3000
```

## Что куда смотреть

| UI | URL | Назначение |
|---|---|---|
| Grafana | http://localhost:3000 | главное окно, datasources уже привязаны |
| Prometheus | http://localhost:9090 | сырые метрики, PromQL |
| Tempo (HTTP API) | http://localhost:3200 | API трейсов, лучше через Grafana |
| Loki (HTTP API) | http://localhost:3100 | API логов, лучше через Grafana |
| OTel Collector | gRPC `:4317`, scrape `:8889` | вход OTLP / выход для Prometheus |

Grafana запускается с анонимным админ-доступом и формой логина выключенной —
поэтому открываешь http://localhost:3000 и сразу попадаешь в админский режим.

## Что собирается

**Traces** — автоматически через `otelgrpc.NewServerHandler()` в gRPC сервере.
Каждый RPC даёт span с именем `sso.v1.AuthService/Login` и т.п.

**Metrics** — тоже автоматом из otelgrpc:
- `rpc_server_call_duration_seconds` — гистограмма latency по методам/кодам
- `rpc_server_calls_total` — счётчик вызовов

**Logs** — `log/slog` через `otelslog`-bridge превращается в OTLP log records.
Каждая запись несёт `trace_id` / `span_id` из активного span'а, поэтому из логов
можно прыгнуть в трейс одним кликом (datasource Loki настроен с `derivedFields`).

## Запуск

```bash
task compose:up        # поднимает всё включая observability
```

Полный список контейнеров:
- `sso` — наш сервис
- `sso-postgres` + `sso-seed` — БД и сидер
- `otel-collector` — фан-аут
- `prometheus`, `tempo`, `loki` — backend'ы
- `grafana` — UI

## Быстрый smoke

После `task compose:up` подождать ~5с и дёрнуть пару RPC:

```bash
grpcurl -plaintext -d '{"email":"x@y.com","password":"longenough"}' \
  localhost:44044 sso.v1.AuthService/Register
grpcurl -plaintext -d '{"email":"x@y.com","password":"longenough","app_id":1}' \
  localhost:44044 sso.v1.AuthService/Login
```

Подождать ~10с (батч-экспорт идёт каждые 10с) и в Grafana:

1. **Explore → Loki** — `{service_name="sso"}` покажет все логи.
2. **Explore → Tempo** — Search by service `sso`, увидишь спаны по методам.
3. **Explore → Prometheus** — `rate(rpc_server_call_duration_seconds_count[1m])`
   покажет RPS по методам и кодам.
4. Клик по `trace_id` в логе Loki — откроется соответствующий трейс в Tempo.

## Выключить OTel (для локальной разработки без стека)

```bash
SSO_OTEL_ENABLED=false go run ./cmd/sso
```

Тогда логи идут только в stdout, метрики/трейсы вообще не собираются —
приложение работает идентично, просто без телеметрии.

## Конфиги

- [deploy/otel-collector.yaml](../deploy/otel-collector.yaml) — pipelines OTLP → Tempo/Prom/Loki
- [deploy/prometheus.yaml](../deploy/prometheus.yaml) — scrape otel-collector:8889
- [deploy/tempo.yaml](../deploy/tempo.yaml) — local storage, retention 1h
- [deploy/loki.yaml](../deploy/loki.yaml) — single-binary tsdb, allow_structured_metadata
- [deploy/grafana/datasources.yaml](../deploy/grafana/datasources.yaml) — три datasource + log-to-trace ссылки
