# CSS собирается из шаблонов и встраивается в бинарник через go:embed;
# node нужен только на этом этапе и в итоговый образ не попадает
FROM node:22-alpine AS css

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY tailwind.config.js tailwind.daisyui.config.js ./
COPY web/styles web/styles
COPY web/templates web/templates
RUN mkdir -p web/static && npm run build:css

FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=css /app/web/static/daisyui.css /app/web/static/tailwind.css web/static/
RUN CGO_ENABLED=0 GOOS=linux go build -o wimm ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /app/wimm .

EXPOSE 8080
CMD ["./wimm"]
