FROM golang:1.25 AS builder

ARG SERVICE_NAME
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY services/shared/ services/shared/
COPY services/${SERVICE_NAME}/ services/${SERVICE_NAME}/

WORKDIR /app/services/${SERVICE_NAME}
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/service ./cmd/main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/service .
EXPOSE 8080
ENTRYPOINT ["./service"]
