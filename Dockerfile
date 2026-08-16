FROM --platform=$BUILDPLATFORM golang:1.26 AS builder

WORKDIR /build

COPY go.mod ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /patrol-dispatch ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /patrol-dispatch /usr/local/bin/patrol-dispatch
COPY config.json /config.json

ENV PATROL_CONFIG=/config.json

EXPOSE 53649

ENTRYPOINT ["/usr/local/bin/patrol-dispatch"]
