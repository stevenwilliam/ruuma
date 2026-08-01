# ── build ─────────────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/ruuma ./cmd/api

# ── web ───────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ── runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN adduser -D -u 10001 ruuma && apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=build /out/ruuma /app/ruuma
COPY --from=web /web/dist /app/web
USER ruuma
EXPOSE 8080
ENTRYPOINT ["/app/ruuma"]
CMD ["serve"]
