FROM public.ecr.aws/docker/library/golang:1.22.12-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server .

FROM public.ecr.aws/docker/library/alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=builder /server /app/server
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
