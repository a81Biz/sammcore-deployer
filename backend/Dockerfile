FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/deployer-backend .

FROM alpine:3.20

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/deployer-backend /app/deployer-backend

EXPOSE 8080

CMD ["/app/deployer-backend"]
