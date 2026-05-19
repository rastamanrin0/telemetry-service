FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY telemetry-platform .

EXPOSE 8080
ENTRYPOINT ["./telemetry-platform"]
