# gateway/Dockerfile

FROM golang:1.25.8-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o gateway-app ./gateway/cmd/main.go

CMD ["./gateway-app"]
