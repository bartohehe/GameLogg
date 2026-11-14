# syntax=docker/dockerfile:1

# Obraz wieloarchitekturowy (linux/amd64, linux/arm64) bez emulacji:
# frontend i kompilacja Go biegną na architekturze maszyny budującej
# ($BUILDPLATFORM), a Go kompiluje krzyżowo na docelową ($TARGETARCH).

# 1. Frontend (wynik nie zależy od architektury)
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Binarka Go z wbudowanym frontendem
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/gamelogg ./cmd/gamelogg

# 3. Obraz końcowy: sama binarka, bez powłoki, jako użytkownik bez uprawnień
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gamelogg /gamelogg
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD ["/gamelogg", "healthcheck"]
ENTRYPOINT ["/gamelogg"]
