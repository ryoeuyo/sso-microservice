# syntax=docker/dockerfile:1.7

# ---------- builder ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Сначала только go.mod/go.sum — слой кэшируется, пока зависимости не менялись.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO выкл — даст статический бинарь для distroless/static.
# trimpath + ldflags чтобы убрать пути сборки и таблицу символов.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/sso ./cmd/sso

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/sso /sso
# Конфиг кладём для случая, когда хочется запускать без env-only.
COPY --from=builder /src/config /config

USER nonroot:nonroot
EXPOSE 44044

ENTRYPOINT ["/sso"]
