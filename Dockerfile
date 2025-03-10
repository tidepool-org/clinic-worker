# Development
FROM golang:1.24.1-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic-worker
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic-worker
USER tidepool
RUN go install github.com/air-verse/air@v1.61.7
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.24.1-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic-worker
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic-worker
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["./dist/worker"]
