# Stage 1: Build admin SPA
FROM node:20-alpine AS admin-builder
WORKDIR /app/web/admin
COPY web/admin/package*.json ./
RUN npm ci
COPY web/admin/ .
RUN npm run build

# Stage 2: Build Go binary with embedded admin UI
FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=admin-builder /app/web/admin/dist /app/internal/httpserver/adminui
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/llm-gateway ./cmd/server

# Stage 3: Minimal runtime
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/llm-gateway .
EXPOSE 8080
ENTRYPOINT ["./llm-gateway"]
