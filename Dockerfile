### STAGE 1: Build ###

FROM golang:buster as builder

RUN mkdir -p /app/src/github.com/DRuggeri/bind_query_exporter
ENV GOPATH /app
WORKDIR /app
COPY . /app/src/github.com/DRuggeri/bind_query_exporter
RUN cd /app/src/github.com/DRuggeri/bind_query_exporter && go install

### STAGE 2: Setup ###

FROM alpine
RUN apk add --no-cache \
  libc6-compat
COPY --from=builder /app/bin/bind_query_exporter /bind_query_exporter
RUN chmod +x /bind_query_exporter
ENTRYPOINT ["/bind_query_exporter"]
