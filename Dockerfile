FROM golang:1.25.4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o buildApp ./cmd/app

EXPOSE 8080

# Команда запуска
CMD ["./buildApp"]