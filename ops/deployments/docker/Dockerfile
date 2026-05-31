FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN go test ./... && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/labelserver ./cmd/labelserver

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/labelserver /app/labelserver
EXPOSE 7870
ENTRYPOINT ["/app/labelserver"]
CMD ["-addr", "0.0.0.0:7870", "-merge-root", "/data/merge", "-frame-root", "/data/frames", "-annotation-root", "/data/merge/annotations_review"]
