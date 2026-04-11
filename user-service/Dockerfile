# user-service/Dockerfile

FROM golang:1.25.8-alpine

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o user ./user-service/cmd/main.go

CMD ["./user"]