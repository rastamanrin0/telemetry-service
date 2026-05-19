# Платформа логов и метрик

Сервис для сбора, хранения и поиска телеметрии распределённых систем.

**Стек:** Go · Elasticsearch · ClickHouse

## Архитектура

```
HTTP API (gin :8080)
    ├── POST /api/v1/logs       → Elasticsearch  (full-text поиск)
    ├── GET  /api/v1/logs       → Elasticsearch
    ├── POST /api/v1/metrics    → ClickHouse      (временные ряды)
    └── GET  /api/v1/metrics    → ClickHouse

Retention Manager (горутина) — применяет политики archive/short/delete
```

## Запуск

### 1. Зависимости

```bash
go mod tidy
```

### 2. Сборка бинаря

В случае мака на арме надо использовать эту сборку, на линуксе, можно сразу переходить к докеру
```bash
GOOS=linux GOARCH=arm64 go build -o telemetry-platform ./cmd/server
```

### 3. Запуск инфраструктуры + приложения

```bash
docker-compose build app
docker-compose up
```

Альтернатива:

```bash
docker-compose up elasticsearch clickhouse
go run ./cmd/server
```

### Проверка

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## API

### Логи

#### Отправить лог

```
POST /api/v1/logs
```

```json
{
  "service_name": "payments",
  "host_id": "host-1",
  "instance_id": "pod-abc",
  "level": "ERROR",
  "message": "connection refused to postgres",
  "retention_policy": "archive"
}
```

Поля `level`: `DEBUG` `INFO` `WARNING` `ERROR` `FATAL`  
Поля `retention_policy`: `archive` (перенести в архив) · `short` (удалить)

Ответ: `202 Accepted` `{"id": "<uuid>"}`

#### Отправить батч логов

```
POST /api/v1/logs/batch
```

```json
{
  "logs": [
    {"service_name": "auth", "host_id": "host-2", "level": "INFO", "message": "user logged in", "retention_policy": "short"},
    {"service_name": "auth", "host_id": "host-2", "level": "WARNING", "message": "invalid token", "retention_policy": "short"}
  ]
}
```

#### Поиск логов

```
GET /api/v1/logs/search
```

| Параметр | Описание |
|---|---|
| `from` | Начало периода (RFC3339) |
| `to` | Конец периода (RFC3339) |
| `service` | Фильтр по service_name |
| `host` | Фильтр по host_id |
| `level` | Фильтр по уровню |
| `retention_policy` | `archive` или `short` |
| `q` | Полнотекстовый поиск по message |
| `page` | Страница (от 0) |
| `size` | Размер страницы (дефолт 100) |

```bash
curl "http://localhost:8080/api/v1/logs/search?service=payments&level=ERROR&q=timeout"
```

#### Статистика по уровням

```
GET /api/v1/logs/stats?service=payments
```

```json
{"counts": {"ERROR": 42, "INFO": 1500, "WARNING": 23}}
```

---

### Метрики

#### Отправить метрику

```
POST /api/v1/metrics
```

```json
{
  "metric_name": "cpu.usage",
  "value": 87.5,
  "tags": {
    "service": "payments",
    "host": "host-1",
    "region": "ru-central"
  }
}
```

#### Отправить батч метрик

```
POST /api/v1/metrics/batch
```

```json
{
  "metrics": [
    {"metric_name": "cpu.usage",    "value": 87.5, "tags": {"service": "payments"}},
    {"metric_name": "memory.usage", "value": 62.1, "tags": {"service": "payments"}}
  ]
}
```

#### Запрос временного ряда

```
GET /api/v1/metrics/query
```

| Параметр | Описание |
|---|---|
| `metric` | Имя метрики **(обязательный)** |
| `from` | Начало периода (RFC3339) |
| `to` | Конец периода (RFC3339) |
| `agg` | Агрегация: `min` `max` `avg` `sum` |
| `window` | Размер окна: `1m` `5m` `1h` и т.д. |
| любой другой | Фильтр по тегу (`service=payments`) |

```bash
# Средняя загрузка CPU по 5-минутным окнам
curl "http://localhost:8080/api/v1/metrics/query?metric=cpu.usage&service=payments&agg=avg&window=5m"
```

```json
{
  "metric_name": "cpu.usage",
  "tags": {"service": "payments"},
  "points": [
    {"timestamp": "2024-01-01T10:00:00Z", "value": 72.3},
    {"timestamp": "2024-01-01T10:05:00Z", "value": 85.1}
  ]
}
```

---

### Health

```
GET /health
→ {"status": "ok"}
```

## Примеры curl

```bash
# Записать лог
curl -s -X POST http://localhost:8080/api/v1/logs \
  -H "Content-Type: application/json" \
  -d '{"service_name":"payments","host_id":"host-1","level":"ERROR","message":"db timeout","retention_policy":"archive"}' | jq

# Поиск ERROR-логов за сутки
curl -s "http://localhost:8080/api/v1/logs/search?level=ERROR&size=20" | jq

# Записать метрику
curl -s -X POST http://localhost:8080/api/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{"metric_name":"http.rps","value":1250,"tags":{"service":"api-gateway"}}' | jq

# Запрос метрики с агрегацией
curl -s "http://localhost:8080/api/v1/metrics/query?metric=http.rps&service=api-gateway&agg=max&window=1m" | jq
```
