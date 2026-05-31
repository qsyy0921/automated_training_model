FROM node:22-bookworm AS web-builder

WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
COPY --from=web-builder /src/web/dist ./web/dist
RUN go test ./... && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/labelserver ./cmd/labelserver

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/labelserver /app/labelserver
COPY --from=web-builder /src/web/dist /app/web/dist
EXPOSE 7870
ENTRYPOINT ["/app/labelserver"]
CMD ["-addr", "0.0.0.0:7870", "-merge-root", "/data/merge", "-frame-root", "/data/frames", "-annotation-root", "/data/merge/annotations_review", "-web-root", "/app/web"]
