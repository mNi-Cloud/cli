FROM golang:1.22.4 AS builder

WORKDIR /app

COPY . .

WORKDIR /app/client

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /mni-client cmd/mni-client/main.go

FROM gcr.io/distroless/base-debian11 AS runner

WORKDIR /

COPY --from=builder /mni-client /mni-client

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/mni-client"]
