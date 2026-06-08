FROM golang:1.26.2 AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -v -o huma

FROM alpine:3.23.3
COPY --from=builder /app/huma /
ENTRYPOINT ["/huma"]