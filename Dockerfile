FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

# Completely disable Go module checksum verification
RUN go env -w GONOSUMCHECK=* && \
    go env -w GONOSUMDB=* && \
    go env -w GOFLAGS=-mod=mod && \
    go env -w GOPRIVATE=*

COPY go.mod ./
RUN rm -f go.sum

RUN go mod download

COPY . .
RUN rm -f go.sum && CGO_ENABLED=0 GOOS=linux go build -mod=mod -o server ./cmd/main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
