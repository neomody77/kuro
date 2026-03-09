# Stage 1: Build frontend
FROM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOTOOLCHAIN=local go mod download
COPY . .
# Embed the built UI into the binary via //go:embed ui in cmd/kuro/main.go
COPY --from=ui-builder /app/ui/dist ./cmd/kuro/ui
RUN CGO_ENABLED=0 GOTOOLCHAIN=local go build -o /kuro ./cmd/kuro/

# Stage 3: Minimal runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata git \
    && adduser -D -h /home/kuro kuro

# Kuro reads config from $HOME/.kuro/config.yaml and defaults DataDir to $HOME/.kuro.
# Set HOME so that /data becomes the effective data directory via config.
RUN mkdir -p /home/kuro/.kuro /data \
    && chown -R kuro:kuro /home/kuro /data

COPY --from=go-builder /kuro /usr/local/bin/kuro

# Write a default config pointing DataDir at /data
RUN printf 'server:\n  host: "0.0.0.0"\n  port: 8080\ndata_dir: /data\n' \
    > /home/kuro/.kuro/config.yaml \
    && chown kuro:kuro /home/kuro/.kuro/config.yaml

USER kuro
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["kuro"]
