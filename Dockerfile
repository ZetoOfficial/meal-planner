# ---- build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /src

# Кэшируем зависимости отдельно от исходников.
COPY go.mod go.sum* ./
# replace => ./api/sdk/go требует модуль SDK в build-контексте до go mod download.
COPY api/sdk/go/go.mod api/sdk/go/go.sum* ./api/sdk/go/
RUN go mod download

COPY . .
# CGO выключен — статический бинарь под scratch/distroless.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/api

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
# Спека встроена в бинарь (embedded-spec), но кладём и файлы для tern/справки.
COPY --from=build /src/docs /app/docs
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
