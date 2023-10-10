# Development
FROM golang:1.21.2-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic-worker
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic-worker
USER tidepool
RUN go install github.com/cosmtrek/air@latest
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.21.2-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic-worker
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic-worker
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["./dist/worker"]
