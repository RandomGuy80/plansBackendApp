FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod ./

# Disable checksum verification completely
ENV GONOSUMCHECK=*
ENV GONOSUMDB=*
ENV GOPRIVATE=*
ENV GOFLAGS=-mod=mod
ENV GONOSUMCHECK=*

# Download dependencies (no go.sum needed)
RUN GONOSUMCHECK=* GONOSUMDB=* GOFLAGS=-mod=mod go mod download

COPY . .

# Build (skip go.sum entirely)
RUN GONOSUMCHECK=* GONOSUMDB=* CGO_ENABLED=0 GOOS=linux go build -mod=mod -o server ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
