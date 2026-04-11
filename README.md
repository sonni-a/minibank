# Minibank (Go, PostgreSQL, gRPC, Redis, JWT, Docker, Docker Compose)

## Функциональность
- Регистрация и авторизация пользователей
- Генерация JWT access и refresh токенов
- CRUD операции с пользователями
- Переводы и платежи между пользователями

## Архитектура
Каждый сервис (auth, user, payment) работает отдельно, общается через gRPC, данные хранятся в PostgreSQL, Redis используется для кэша токенов.

## Инструкция по запуску
1. Склонировать репозиторий
```bash
git clone github.com/sonni-a/minibank.git
```
2. Создать .env на основе .env.example
```bash
cp .env.example .env
```
3. Запустить через docker-compose
```bash
docker-compose up --build
```

## Запросы и примеры 
Register:

<img width="732" height="174" alt="image" src="https://github.com/user-attachments/assets/a630222e-d6d0-45b0-bb41-5af5e5a44276" />

Login:

<img width="825" height="168" alt="image" src="https://github.com/user-attachments/assets/31681b29-6562-4bf6-866b-3c468f9c61a1" />

Create User:

<img width="849" height="187" alt="image" src="https://github.com/user-attachments/assets/4a521777-740b-4c45-8857-9275a9814e5a" />

Update User:

<img width="858" height="176" alt="image" src="https://github.com/user-attachments/assets/93386176-3c8b-4118-ba0b-8054fe3ab320" />

Get User:

<img width="858" height="172" alt="image" src="https://github.com/user-attachments/assets/38a3b05f-163b-4b7b-895e-c2406c8ae074" />

Delete User:

<img width="854" height="154" alt="image" src="https://github.com/user-attachments/assets/afc85ff0-b269-4150-9ac9-78846035a046" />

Create Account (создать счет):

<img width="858" height="154" alt="image" src="https://github.com/user-attachments/assets/2e052507-6edd-4fad-a2c1-1ee97458254d" />

Deposit (пополнить счет):

<img width="855" height="151" alt="image" src="https://github.com/user-attachments/assets/b8a602f0-7b83-481b-9b10-d6dbd8de3a61" />

Get Balance (узнать баланс):

<img width="856" height="152" alt="image" src="https://github.com/user-attachments/assets/86e054a7-c8c1-4d25-ba0b-6e4367813336" />

Transfer (перевести с одного счета на другой):

<img width="849" height="154" alt="image" src="https://github.com/user-attachments/assets/dba5a37c-cdbb-4988-8e3d-525199a0c4c3" />

Попытка перевести сумму больше той, которая находится на счете:

<img width="851" height="142" alt="image" src="https://github.com/user-attachments/assets/649dfe2f-2f57-49e0-a334-b597f42fe76d" />
