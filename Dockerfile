FROM public.ecr.aws/docker/library/golang:1.27.1 AS builder

ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/aws-metrics-exporter

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /out/aws-metrics-exporter /app/aws-metrics-exporter

EXPOSE 8080

ENTRYPOINT ["/app/aws-metrics-exporter"]
CMD ["worker", "--config", "/app/config.yaml"]
