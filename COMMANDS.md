# FrimoMessenger — команды для запуска и разработки

Этот файл — шпаргалка по основным командам проекта.

Проект поддерживает два режима запуска:

1. **Full Docker** — Nginx, API, Worker и PostgreSQL работают в контейнерах.
2. **Local Development** — Nginx и PostgreSQL работают в Docker, а API и Worker запускаются локально из IDE.

---

## 1. Full Docker

Используется, когда нужно проверить приложение в окружении, максимально похожем на production.

### Запустить весь стек

```bash
docker compose up -d --build
```

Что делает команда:

- `docker compose` — запускает Docker Compose;
- `up` — создаёт и запускает сервисы;
- `-d` — запускает их в фоне;
- `--build` — перед запуском пересобирает образы, если код изменился.

Будут запущены:

```text
Nginx
  ↓
API
  ↓
PostgreSQL

Worker
  ↓
PostgreSQL
```

### Остановить весь стек

```bash
docker compose down
```

Останавливает контейнеры и удаляет Compose network.

Данные PostgreSQL при этом сохраняются, потому что они находятся в named volume.

> Не добавляй `-v`, если не хочешь удалить данные локальной базы.

### Пересобрать Docker images

```bash
docker compose build
```

Полная пересборка без Docker cache:

```bash
docker compose build --no-cache
```

### Посмотреть состояние контейнеров

```bash
docker compose ps
```

Позволяет проверить статус сервисов, опубликованные порты и состояние healthcheck.

### Смотреть логи всех сервисов

```bash
docker compose logs -f
```

`-f` означает `follow`: терминал продолжает показывать новые сообщения. Остановить просмотр можно через `Ctrl + C`; контейнеры при этом не останавливаются.

### Смотреть логи конкретного сервиса

API:

```bash
docker compose logs -f api
```

Worker:

```bash
docker compose logs -f worker
```

Nginx:

```bash
docker compose logs -f nginx
```

PostgreSQL:

```bash
docker compose logs -f postgres
```

---

## 2. Local Development

В этом режиме инфраструктура работает в Docker, а Go-приложение запускается напрямую из GoLand.

```text
Client
  ↓
Nginx container
  ↓
Go API в IDE
  ↓
PostgreSQL container

Go Worker в IDE
  ↓
PostgreSQL container
```

Используются два Compose-файла:

```text
compose.yaml
compose.dev.yaml
```

Compose объединяет их слева направо:

```bash
docker compose -f compose.yaml -f compose.dev.yaml ...
```

`compose.dev.yaml` переопределяет нужные настройки базового `compose.yaml`.

### Посмотреть итоговую dev-конфигурацию

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  config
```

Команда ничего не запускает. Она показывает итоговую Compose-конфигурацию после объединения двух YAML-файлов.

Полезно проверять:

- какой Nginx config будет подключён;
- остался ли `depends_on`;
- какие environment variables используются;
- какие volumes и ports получились после merge.

> `docker compose config` может вывести реальные значения переменных из `.env`, включая секреты.

### Запустить инфраструктуру для локальной разработки

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  up -d postgres nginx
```

Запускаются только `postgres` и `nginx`. API и Worker запускаются из IDE.

### Запуск API из IDE

Запускается пакет:

```text
./cmd/server
```

Локальный API использует PostgreSQL через:

```text
localhost:5432
```

а не через `postgres:5432`, потому что имя `postgres` существует внутри Docker network.

### Запуск Worker из IDE

Запускается пакет:

```text
./cmd/worker
```

Worker использует тот же локальный PostgreSQL через `localhost:5432`.

API и Worker можно запускать независимо, что удобно при отладке HTTP или outbox.

### Остановить dev-инфраструктуру

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  down
```

Данные PostgreSQL сохранятся, если не добавлять `-v`.

### Посмотреть состояние dev-контейнеров

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  ps
```

### Смотреть dev-логи

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  logs -f postgres nginx
```

---

## 3. PostgreSQL

PostgreSQL опубликован на host-машину через:

```text
127.0.0.1:5432
```

Поэтому адрес зависит от того, откуда подключается приложение.

Из контейнера:

```text
postgres:5432
```

Из GoLand / локального Go-процесса:

```text
localhost:5432
```

---

## 4. Миграции

Сервис `migrate` находится в Compose profile `tools`.

### Применить все новые миграции

```bash
docker compose --profile tools run --rm migrate up
```

`run` создаёт отдельный container для команды, а `--rm` удаляет его после завершения.

### Откатить одну миграцию

```bash
docker compose --profile tools run --rm migrate down 1
```

Откатывает только последнюю применённую миграцию.

---

## 5. Nginx

### Проверить конфигурацию Nginx

```bash
docker compose exec nginx nginx -t
```

При корректной конфигурации Nginx должен вывести примерно:

```text
syntax is ok
test is successful
```

### Перезагрузить Nginx без пересоздания container

```bash
docker compose exec nginx nginx -s reload
```

Nginx перечитает конфигурацию без полного restart container.

### Проверить health endpoint через Nginx

```bash
curl -i http://localhost/health
```

Ожидается `HTTP/1.1 200 OK` и JSON:

```json
{"status":"ok"}
```

---

## 6. Healthcheck API

Docker проверяет API внутри самого container через:

```text
http://localhost:8080/health
```

Проверить endpoint непосредственно из API container можно так:

```bash
docker compose exec api wget -qO- http://localhost:8080/health
```

---

## 7. Проверка rate limiting

Для `/auth/resend` можно быстро отправить несколько запросов подряд:

```bash
for i in $(seq 1 10); do
  curl -i \
    -X POST \
    -H 'Content-Type: application/json' \
    -d '{"email":"test@example.com"}' \
    http://localhost/auth/resend
done
```

Когда Nginx rate limiter сработает, ожидается:

```text
HTTP/1.1 429 Too Many Requests
```

и JSON:

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "too many requests"
  }
}
```

---

## 8. Ежедневный workflow

### Обычная разработка

Запустить инфраструктуру:

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  up -d postgres nginx
```

Затем запустить из GoLand:

```text
Run/Debug ./cmd/server
Run/Debug ./cmd/worker
```

После работы:

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  down
```

### Проверка полного Docker-варианта

```bash
docker compose up -d --build
```

Проверить состояние:

```bash
docker compose ps
```

Посмотреть логи:

```bash
docker compose logs -f
```

После проверки:

```bash
docker compose down
```

---

## 9. Команды, с которыми нужно быть осторожным

### Удаление volumes

```bash
docker compose down -v
```

`-v` удаляет named volumes. Для PostgreSQL это означает удаление локальных данных базы.

### Просмотр Compose config

```bash
docker compose config
```

или:

```bash
docker compose \
  -f compose.yaml \
  -f compose.dev.yaml \
  config
```

Команда может вывести раскрытые значения из `.env`, включая пароли и encryption keys. Такой вывод не стоит публиковать или сохранять в Git.

---

## 10. Краткая шпаргалка

| Задача | Команда |
|---|---|
| Запустить полный Docker stack | `docker compose up -d --build` |
| Остановить полный stack | `docker compose down` |
| Запустить dev-инфраструктуру | `docker compose -f compose.yaml -f compose.dev.yaml up -d postgres nginx` |
| Остановить dev-инфраструктуру | `docker compose -f compose.yaml -f compose.dev.yaml down` |
| Посмотреть контейнеры | `docker compose ps` |
| Смотреть все логи | `docker compose logs -f` |
| Смотреть API logs | `docker compose logs -f api` |
| Смотреть Worker logs | `docker compose logs -f worker` |
| Проверить Nginx config | `docker compose exec nginx nginx -t` |
| Reload Nginx | `docker compose exec nginx nginx -s reload` |
| Применить миграции | `docker compose --profile tools run --rm migrate up` |
| Откатить одну миграцию | `docker compose --profile tools run --rm migrate down 1` |
| Проверить dev Compose merge | `docker compose -f compose.yaml -f compose.dev.yaml config` |
| Проверить API health | `curl -i http://localhost/health` |
