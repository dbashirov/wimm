FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wimm ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /app/wimm .

EXPOSE 8080
CMD ["./wimm"]
