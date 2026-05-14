# Информационная система бронирования мест в коворкинге

Учебный веб-прототип на **Go + PostgreSQL**, оформленный под мобильный интерфейс Telegram Mini App (карточки, крупные кнопки, нижняя навигация). Запускается одним `docker compose`.

## Стек

- **Backend:** Go 1.22 (стандартная библиотека: `net/http`, `html/template`)
- **БД:** PostgreSQL 16
- **Запуск:** Docker + docker-compose
- **Авторизация:** email + пароль, bcrypt-хеши, cookie-сессии с HMAC-подписью

## Быстрый старт

```bash
docker compose up --build
```

После старта:

- Приложение: <http://localhost:8080>
- PostgreSQL: `localhost:5432`, БД `coworking`, пользователь `coworking`, пароль `coworking`

Healthcheck приложения: `GET /healthz` → `ok`.

## Учётные данные по умолчанию

| Роль | Email | Пароль |
|---|---|---|
| Администратор | `admin@example.com` | `admin` |

Пароль хранится в БД только в виде bcrypt-хеша; создаётся скриптом `db/init/02_seed.sql` при первом запуске.

Обычный пользователь создаётся через форму регистрации на `/register`.

## Остановка и сброс

```bash
# Остановить контейнеры (данные БД сохраняются в томе db_data):
docker compose down

# Полная очистка вместе с базой:
docker compose down -v
```

## Переменные окружения

Все значения заданы в `docker-compose.yml`. При необходимости можно переопределить через `.env` или `-e`:

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `DB_HOST` / `DB_PORT` | `db` / `5432` | Адрес PostgreSQL |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `coworking` × 3 | Креды БД |
| `SESSION_SECRET` | `change-me-in-production` | Секрет HMAC-подписи cookie-сессий |
| `DOCKER_REGISTRY` | `mirror.gcr.io` | Зеркало Docker-образов (на случай блокировки docker.io) |

## Маршруты

**Пользовательские:**

- `/` — главная
- `/login`, `/register`, `/logout` — авторизация
- `/scheme` — схема коворкинга, выбор даты/интервала, бронирование
- `/bookings` — мои бронирования + отмена
- `/settings` — личные настройки
- `/workspaces/history` — история бронирований места

**Админские** (только для `admin@example.com` или роли `ADMIN`):

- `/admin` — панель с местами, бронированиями и настройками
- `/admin/coworkings/...` — CRUD коворкингов
- `/admin/workspaces/...` — CRUD мест (create / update / toggle / delete)
- `/admin/bookings/cancel`, `/admin/bookings/status` — управление бронированиями
- `/admin/settings` — изменение `max_active_bookings_per_user`
- `/admin/report` — отчёт за период (сводка, топ-места, загрузка по дням, активность пользователей)

## База данных

Схема и начальные данные создаются автоматически из `db/init/01_schema.sql` и `db/init/02_seed.sql`. Сущности (по `.md`-спецификации):

`users`, `coworkings`, `workspaces`, `bookings`, `booking_settings`, `reports`, `notifications`.

Enum-типы: `role`, `workspace_type`, `booking_status`, `notification_type`.

Подключиться к БД для ручной проверки:

```bash
docker exec -it coworking_db psql -U coworking -d coworking
```

## Структура проекта

```
cmd/server/         точка входа (main.go)
internal/
  auth/             bcrypt + cookie-сессии
  db/               подключение к PostgreSQL
  handlers/         HTTP-обработчики (auth, scheme, bookings, admin, report)
  models/           структуры моделей и enum-значения
  repo/             репозитории для доступа к таблицам
db/init/            SQL-миграции и сидинг (создаются при первом запуске БД)
web/templates/      HTML-шаблоны (html/template)
web/static/         CSS и статика
docker-compose.yml  сервисы app + db
Dockerfile          сборка Go-приложения (multi-stage)
```

## Полная спецификация

Подробная предметная модель, use cases и бизнес-правила — в файле [`full_description_cowork (2).md`](./full_description_cowork%20\(2\).md).
