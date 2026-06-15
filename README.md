# Minibank

Микросервисный backend банка на Go.
Проект моделирует базовые банковские сценарии: регистрацию, работу с профилем, счетом, балансом и переводами.

## Стек

- Go
- gRPC + Protocol Buffers
- PostgreSQL
- Redis
- JWT (access + refresh)
- Docker / Docker Compose
- golang-migrate

## Архитектура

- `auth-service` - регистрация, логин, refresh токена
- `user-service` - операции с профилем пользователя
- `payment-service` - счет, баланс, пополнение, перевод
- `gateway` - HTTP-вход и взаимодействие между gRPC-сервисами

## Функциональность

- Регистрация и авторизация пользователей
- Генерация и валидация access/refresh JWT
- CRUD для пользователей
- Создание счета
- Пополнение счета
- Получение баланса
- Перевод между счетами с транзакцией

## Как запустить проект

1. Склонировать репозиторий:

```bash
git clone github.com/sonni-a/minibank.git
cd minibank
```

2. Создать `.env` из примера:

```bash
cp .env.example .env
```

3. Запустить все сервисы:

```bash
docker compose up --build
```

4. Проверить, что gateway поднялся:

```bash
curl http://localhost:8080/health
```

После запуска доступны:

- gateway: `localhost:8080`
- auth-service (gRPC): `localhost:50051`
- user-service (gRPC): `localhost:50052`
- payment-service (gRPC): `localhost:50053`

## HTTP API (gateway :8080)

### Публичные роуты

Проверка здоровья:
```bash
curl http://localhost:8080/health
```

Регистрация (создаёт auth-запись, профиль и счёт за один запрос):
```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'
```

Логин:
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'
```

Обновление access-токена:
```bash
curl -X POST http://localhost:8080/api/v1/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<REFRESH_TOKEN>"}'
```

### Защищенные роуты (требуют `Authorization: Bearer <ACCESS_TOKEN>`)

Мой профиль:
```bash
curl http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

Обновить профиль:
```bash
curl -X PUT http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Smith","email":"alice@example.com"}'
```

Удалить аккаунт:
```bash
curl -X DELETE http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

Баланс:
```bash
curl http://localhost:8080/api/v1/balance \
  -H "Authorization: Bearer <ACCESS_TOKEN>"
```

Пополнение счёта (сумма в копейках):
```bash
curl -X POST http://localhost:8080/api/v1/deposit \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"amount_minor":10000}'
```

Перевод:
```bash
curl -X POST http://localhost:8080/api/v1/transfer \
  -H "Authorization: Bearer <ACCESS_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"to_user_id":2,"amount_minor":5000}'
```

### gRPC напрямую (через grpcurl)

```bash
grpcurl -plaintext -d '{"email":"alice@example.com","password":"secret123"}' \
  localhost:50051 auth.AuthService/Login
```



## Тестирование

Запуск всех тестов:

```bash
go test ./...
```
