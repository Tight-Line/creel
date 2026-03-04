FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /creel ./cmd/creel
RUN CGO_ENABLED=0 go build -o /creel-cli ./cmd/creel-cli

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /creel /usr/local/bin/creel
COPY --from=builder /creel-cli /usr/local/bin/creel-cli
COPY migrations/ /etc/creel/migrations/
EXPOSE 8443 8080 9090
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD creel-cli health || exit 1
ENTRYPOINT ["creel"]
