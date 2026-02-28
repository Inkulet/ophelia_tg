# Stage 1: Frontend build
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Backend build (CGO for SQLite)
FROM golang:1.23-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o ophelia_bot main.go

# Stage 3: Final runtime image
FROM alpine:latest
RUN apk add --no-cache sqlite-libs tzdata ca-certificates
WORKDIR /app

COPY --from=backend-builder /app/ophelia_bot ./ophelia_bot
COPY --from=frontend-builder /app/frontend/dist/ophelia/browser ./frontend/dist/ophelia/browser

EXPOSE 8080
CMD ["./ophelia_bot"]
