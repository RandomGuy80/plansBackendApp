FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

RUN go env -w GONOSUMDB=* && \
    go env -w GOFLAGS=-mod=mod && \
    go env -w GOPRIVATE=*

COPY go.mod ./
RUN rm -f go.sum && go mod download

COPY . .
RUN rm -f go.sum && CGO_ENABLED=0 GOOS=linux go build -mod=mod -o server ./cmd/

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]

# force rebuild
