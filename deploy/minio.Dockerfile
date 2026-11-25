# MinIO community images are no longer published; build the pinned upstream source.
FROM public.ecr.aws/docker/library/golang:1.22.12-alpine AS builder
ARG MINIO_RELEASE=RELEASE.2024-10-13T13-34-11Z
RUN CGO_ENABLED=0 GOBIN=/out go install github.com/minio/minio@${MINIO_RELEASE} \
    && cp /go/pkg/mod/github.com/minio/minio@*/LICENSE /out/MINIO-LICENSE

FROM public.ecr.aws/docker/library/alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 minio \
    && mkdir /data && chown minio:minio /data
COPY --from=builder /out/minio /usr/local/bin/minio
COPY --from=builder /out/MINIO-LICENSE /usr/share/licenses/minio/LICENSE
USER minio
EXPOSE 9000 9001
ENTRYPOINT ["/usr/local/bin/minio"]
