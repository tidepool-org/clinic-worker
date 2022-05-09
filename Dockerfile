# Development
FROM golang:1.17-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic-worker
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic-worker
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["./dist/worker"]
