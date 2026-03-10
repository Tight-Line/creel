FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN buf generate && go mod tidy
RUN CGO_ENABLED=0 go build -o /creel ./cmd/creel
RUN CGO_ENABLED=0 go build -o /creel-cli ./cmd/creel-cli
RUN CGO_ENABLED=0 go build -o /creel-chat ./cmd/creel-chat
RUN CGO_ENABLED=0 go build -o /creel-mcp ./cmd/creel-mcp

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
    && addgroup -S creel && adduser -S creel -G creel
COPY --from=builder /creel /usr/local/bin/creel
COPY --from=builder /creel-cli /usr/local/bin/creel-cli
COPY --from=builder /creel-chat /usr/local/bin/creel-chat
COPY --from=builder /creel-mcp /usr/local/bin/creel-mcp
WORKDIR /app
COPY migrations/ /app/migrations/
EXPOSE 8443 8080 9090
USER creel
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD creel-cli health || exit 1
ENTRYPOINT ["creel"]
