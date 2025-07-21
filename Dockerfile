FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o spotiseek ./cmd/spotiseek
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o clear-searches ./cmd/clear-searches

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /src/spotiseek /app/
COPY --from=builder /src/clear-searches /app/
ENTRYPOINT ["/app/spotiseek"]
