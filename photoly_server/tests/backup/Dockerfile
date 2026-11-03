FROM golang:1.26.10-bookworm AS go
FROM pgvector/pgvector:0.8.7-pg17-bookworm
COPY --from=go /usr/local/go /usr/local/go
COPY --from=go /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV PATH=/usr/local/go/bin:/usr/lib/postgresql/17/bin:$PATH CGO_ENABLED=0 GOPATH=/go
WORKDIR /src
ENTRYPOINT []
CMD ["go", "test", "./internal/backup", "-count=1", "-v"]
