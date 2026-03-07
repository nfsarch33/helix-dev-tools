FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o /cursor-tools ./cmd/cursor-tools/

FROM gcr.io/distroless/static-debian12
COPY --from=build /cursor-tools /usr/local/bin/cursor-tools
ENTRYPOINT ["cursor-tools"]
