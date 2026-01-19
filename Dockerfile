# syntax=docker/dockerfile:1

############################
# Build stage
############################
FROM golang:1.25.4-alpine AS builder

WORKDIR /src

# CA certs + git for fetching private/public modules if needed
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static-ish binary (works well with distroless)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/app

############################
# Runtime stage
############################
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/gateway /gateway

# Echo will listen on ADDRESS env (k8s sets it to 0.0.0.0:8000)
EXPOSE 8000

USER nonroot:nonroot

ENTRYPOINT ["/gateway"]
