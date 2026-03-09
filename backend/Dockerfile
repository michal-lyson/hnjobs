FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags "fts5" -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /server .

ENV DB_PATH=/data/jobs.db
ENV ADDR=:8080

VOLUME ["/data"]
EXPOSE 8080

CMD ["./server"]
