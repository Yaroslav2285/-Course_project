# LR #11: Containerization — multi-stage build for minimal image size
# LR #15: SAST — non-root user, distroless base image

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/go-escrow .

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /build/go-escrow /go-escrow
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8081

USER nonroot:nonroot

ENTRYPOINT ["/go-escrow"]
