FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker


FROM alpine:latest

COPY --from=builder /bin/server /usr/local/bin/server
COPY --from=builder /bin/worker /usr/local/bin/worker