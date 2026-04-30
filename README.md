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

## Примеры использования

### 1) HTTP через gateway

Проверка здоровья:

```bash
curl http://localhost:8080/health
```

Регистрация (gateway оркестрирует auth + user + payment):

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Alice\",\"email\":\"alice@example.com\",\"password\":\"secret123\"}"
```

### 2) gRPC через grpcurl

Логин:

```bash
grpcurl -plaintext -d "{\"email\":\"alice@example.com\",\"password\":\"secret123\"}" \
  localhost:50051 auth.AuthService/Login
```

После логина сохранить `token` и использовать его как `Bearer` metadata для защищенных методов:

```bash
grpcurl -plaintext -H "authorization: Bearer <ACCESS_TOKEN>" -d "{}" \
  localhost:50052 user.UserService/GetMyUser
```

Пополнение счета:

```bash
grpcurl -plaintext -H "authorization: Bearer <ACCESS_TOKEN>" \
  -d "{\"amount_minor\":10000}" \
  localhost:50053 payment.PaymentService/Deposit
```

Перевод:

```bash
grpcurl -plaintext -H "authorization: Bearer <ACCESS_TOKEN>" \
  -d "{\"to_user_id\":2,\"amount_minor\":5000}" \
  localhost:50053 payment.PaymentService/Transfer
```

Refresh токена:

```bash
grpcurl -plaintext -d "{\"refresh_token\":\"<REFRESH_TOKEN>\"}" \
  localhost:50051 auth.AuthService/RefreshToken
```

Проверка баланса:

```bash
grpcurl -plaintext -H "authorization: Bearer <ACCESS_TOKEN>" -d "{}" \
  localhost:50053 payment.PaymentService/GetBalance
```



## Тестирование

Запуск всех тестов:

```bash
go test ./...
```
