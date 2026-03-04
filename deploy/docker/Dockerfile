FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /creel ./cmd/creel

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /creel /usr/local/bin/creel
EXPOSE 8443 8080 9090
ENTRYPOINT ["creel"]
