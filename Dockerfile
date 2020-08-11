FROM golang:alpine as builder
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates
ENV USER=webapp
ENV UID=1001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    webapp
WORKDIR $GOPATH/src/welbymcroberts/hub3_exporter/
COPY . .
RUN go get
RUN CGO_ENABLED=0  go build -ldflags="-w -s" -o /go/bin/hub3_exporter
RUN chmod +x /go/bin/hub3_exporter

#############################
FROM scratch
# Import from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /go/bin/hub3_exporter /go/bin/hub3_exporter

USER webapp:webapp
ENTRYPOINT ["/go/bin/hub3_exporter"]