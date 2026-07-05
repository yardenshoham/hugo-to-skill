FROM alpine:3 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=certs /tmp /tmp
COPY hugo-to-skill /
ENTRYPOINT ["/hugo-to-skill"]
