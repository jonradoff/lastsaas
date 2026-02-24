# Stage 1: Build Go backend
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache git
WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY VERSION ./VERSION
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X lastsaas/internal/version.buildVersion=$(cat VERSION)" -o lastsaas ./cmd/server

# Stage 2: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /build/lastsaas ./lastsaas

# Copy prod config
COPY backend/config/prod.yaml ./config/prod.yaml

# Copy frontend dist
COPY --from=frontend-builder /build/dist ./static

ENV LASTSAAS_ENV=prod
EXPOSE 8080

CMD ["./lastsaas"]
