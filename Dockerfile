# Build the program in a Go image, then copy only the program into a small image.

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY static ./static
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /binmonitor .

FROM alpine:3.20
RUN adduser -D -H binmonitor && mkdir /data && chown binmonitor /data
COPY --from=build /binmonitor /usr/local/bin/binmonitor
USER binmonitor

# data.json and readings.csv are kept in /data so they survive a restart
ENV DATA_DIR=/data PORT=8080
VOLUME /data
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO /dev/null http://localhost:8080/health || exit 1

CMD ["binmonitor"]
