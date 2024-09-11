FROM golang:1.23.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o myapp ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/myapp .
COPY --from=builder /app/.env .

COPY --from=builder /app/api/email/template.html ./api/email/

EXPOSE 1234
EXPOSE 2345

CMD ["./myapp"]
