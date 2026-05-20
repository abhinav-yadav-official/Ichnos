FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/crawler ./cmd/crawler

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=builder /out/crawler /crawler

CMD ["/crawler"]
