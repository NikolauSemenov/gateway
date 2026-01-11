FROM golang:1.25.4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o myapp ./cmd/app

EXPOSE 8086

# Команда запуска
CMD ["./myapp"]
