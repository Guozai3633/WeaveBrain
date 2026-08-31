FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /weavebrain ./cmd/weavebrain
RUN CGO_ENABLED=0 go build -o /mcp-notion ./cmd/mcp-notion
RUN CGO_ENABLED=0 go build -o /mcp-email ./cmd/mcp-email

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /weavebrain /usr/local/bin/weavebrain
COPY --from=builder /mcp-notion /usr/local/bin/mcp-notion
COPY --from=builder /mcp-email /usr/local/bin/mcp-email
EXPOSE 8080
CMD ["weavebrain"]
