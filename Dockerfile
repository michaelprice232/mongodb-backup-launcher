FROM golang:1.23 AS builder

WORKDIR /

COPY go.mod .
COPY go.sum .
COPY cmd/ cmd/
COPY config/ config/
COPY internal/ internal/

RUN go mod download

RUN CGO_ENABLED=0 go build -o /mongodb-backup-launcher ./cmd/main.go && chmod a+x /mongodb-backup-launcher

FROM gcr.io/distroless/base
COPY --from=builder /mongodb-backup-launcher /mongodb-backup-launcher
ENTRYPOINT ["/mongodb-backup-launcher"]