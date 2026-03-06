FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /cursor-tools ./cmd/cursor-tools/

FROM gcr.io/distroless/static-debian12
COPY --from=build /cursor-tools /usr/local/bin/cursor-tools
ENTRYPOINT ["cursor-tools"]
