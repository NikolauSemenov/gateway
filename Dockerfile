FROM golang:1.25.4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o collector ./cmd/worker

EXPOSE 8000

CMD ["./collector"]
