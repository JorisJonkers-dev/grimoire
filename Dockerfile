# syntax=docker/dockerfile:1.26
# One image: the Go API serves the built web app (ARCHITECTURE.md §19.1).

# Both build stages run on the build platform; only the Go binary is cross-compiled, so a
# multi-arch build needs no emulation.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
RUN npm install -g pnpm@12.6.0
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY openapi/package.json openapi/package.json
COPY web/package.json web/package.json
RUN pnpm install --frozen-lockfile --filter @grimoire/web...
COPY openapi openapi
COPY fixtures fixtures
COPY web web
RUN pnpm --filter @grimoire/web build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS api
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api .
COPY --from=web /src/web/dist internal/platform/webui/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/grimoire ./cmd/grimoire

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/grimoire /grimoire
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/grimoire"]
CMD ["serve"]
