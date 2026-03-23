# auth-service/Dockerfile

FROM golang:1.25.8-alpine

WORKDIR /app

COPY ../../ ./

RUN go mod download

RUN go build -o auth ./auth-service/cmd/main.go

CMD ["./auth"]