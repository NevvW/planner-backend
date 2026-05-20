FROM golang:1.25.6 AS builder
WORKDIR /src

ARG SERVICE=auth


ENV GOMODCACHE=/go/pkg/mod
ENV GOCACHE=/root/.cache/go-build

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download


COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/service ./cmd/${SERVICE}
	
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/service /app/service
EXPOSE 50051 8080 50052 8081
ENTRYPOINT ["/app/service"]
