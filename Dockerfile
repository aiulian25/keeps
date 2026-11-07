# syntax=docker/dockerfile:1
# Keeps production image: static Go binary on distroless (no shell, no
# package manager), running as UID/GID 74532. Base images are digest-pinned.

FROM node:22-trixie-slim@sha256:be1e2a3dc2c4fd68cdfadad2a696a280e0d2749098bee509230174d72fb768c6 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/webui/dist internal/webui/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/app/keeps ./cmd/keeps \
    && mkdir -p /out/data \
    && chown 74532:74532 /out/data \
    && chmod 0700 /out/data

FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
COPY --from=build /out/app/keeps /app/keeps
COPY --from=build --chown=74532:74532 --chmod=0700 /out/data /data
LABEL org.opencontainers.image.source="https://github.com/aiulian25/keeps" \
      org.opencontainers.image.description="Self-hosted home library for what you own, where it lives and the proof you bought it" \
      org.opencontainers.image.licenses="MIT"
ENV KEEPS_DATA_DIR=/data
USER 74532:74532
EXPOSE 6030
VOLUME ["/data"]
ENTRYPOINT ["/app/keeps"]
CMD ["serve"]
