FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags "-s -w -X kexoz/cmd.Version=${VERSION}" -o /out/kexoz .

FROM alpine:3.22
LABEL org.opencontainers.image.source="https://github.com/wrenyn/kexoz-cli" \
      org.opencontainers.image.description="Kexoz CLI - share local ports through a public URL"
RUN apk add --no-cache ca-certificates
COPY --from=build /out/kexoz /usr/local/bin/kexoz
ENTRYPOINT ["kexoz"]
