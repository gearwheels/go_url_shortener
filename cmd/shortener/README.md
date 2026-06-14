# cmd/shortener

Точка входа сервиса сокращения URL. Инициализирует зависимости, настраивает маршрутизацию и запускает HTTP-сервер.

## Запуск

```sh
go run ./cmd/shortener/
```

### Флаги командной строки

| Флаг | По умолчанию | Описание |
| ---- | ----------- | -------- |
| `-a` | `localhost:8080` | Адрес HTTP-сервера |
| `-b` | `http://localhost:8080/` | Публичный базовый URL коротких ссылок |
| `-f` | `./storage/store_url.txt` | Путь к файлу-хранилищу (in-memory режим) |
| `-d` | `postgres://...` | DSN для PostgreSQL (пустая строка → in-memory) |
| `-k` | `` | Секретный ключ HMAC для cookie-аутентификации |
| `-s` | `false` | Включить HTTPS (самоподписанный сертификат) |
| `-t` | `` | CIDR доверенной подсети для `/api/internal/stats` |
| `-audit-file` | `` | Путь к файлу аудита событий |
| `-audit-url` | `` | URL удалённого приёмника событий |
| `-c`/`-config` | `` | Путь к файлу конфигурации JSON |

Переменная окружения `CONFIG` имеет приоритет над флагом `-c`/`-config`.

### Переменные окружения

`SERVERADDRESS`, `BASEURL`, `FILE_STORAGE_PATH`, `DATABASE_DSN`, `SECRET_KEY_FOR_JWT`,
`ENABLE_HTTPS`, `TRUSTED_SUBNET`, `AUDIT_FILE`, `AUDIT_URL`, `CONFIG`.

Переменные окружения имеют наивысший приоритет над флагами и файлом конфигурации.

### Файл конфигурации JSON

```json
{
  "server_address": "localhost:8080",
  "base_url": "http://localhost:8080/",
  "file_storage_path": "./storage/store_url.txt",
  "database_dsn": "",
  "secret_key": "",
  "enable_https": false,
  "trusted_subnet": "192.168.1.0/24",
  "audit_file": "",
  "audit_url": ""
}
```

## HTTP-эндпоинты

| Метод | Путь | Описание |
| ----- | ---- | -------- |
| `POST` | `/` | Сокращение URL (text/plain) |
| `POST` | `/api/shorten` | Сокращение URL (JSON) |
| `POST` | `/api/shorten/batch` | Пакетное сокращение (JSON) |
| `GET` | `/{id}` | Редирект на оригинальный URL |
| `GET` | `/ping` | Проверка доступности БД |
| `GET` | `/api/user/urls` | Список ссылок текущего пользователя |
| `DELETE` | `/api/user/urls` | Пометить ссылки как удалённые |
| `GET` | `/api/internal/stats` | Статистика (только из доверенной подсети) |

## Graceful shutdown

Сервер корректно завершается по сигналам `SIGTERM`, `SIGINT`, `SIGQUIT`:

1. `http.Server.Shutdown` ожидает завершения всех активных запросов (до 30 с).
2. Воркер удаления URL дожидается опустошения очереди задач.
