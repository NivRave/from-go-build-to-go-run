# syntax=docker/dockerfile:1
# Build from the repository root, so go.mod and go.sum are in the context:
#
#   docker build -f examples/059-docker/Dockerfile -t gosvc:distroless .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./examples/059-docker

# distroless/static already contains CA certificates, time zone data,
# /etc/passwd and the nonroot user (UID 65532); the :nonroot tag runs as it.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
ENTRYPOINT ["/app"]
