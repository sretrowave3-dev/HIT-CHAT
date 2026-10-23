# Stage 1: Build Web Client Assets
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Backend Server Binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /campusnet-server ./cmd/server

# Stage 3: Minimal Production Image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

# Non-root security user
RUN addgroup -S campusnet && adduser -S campusnet -G campusnet

COPY --from=go-builder /campusnet-server /app/campusnet-server
COPY --from=web-builder /app/web/dist /app/web/dist

USER campusnet
EXPOSE 8080

ENTRYPOINT ["/app/campusnet-server"]
CMD ["-port=8080"]
