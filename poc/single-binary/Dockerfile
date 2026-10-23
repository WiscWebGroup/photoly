# syntax=docker/dockerfile:1
#
# Builds the POC into a ~20 MB image: one static binary, CA certificates, nothing else.
# Behind a TLS-intercepting proxy, pass its CA so npm and Go can download:
#   docker build --secret id=extra_ca,src=/path/to/proxy-ca.crt .

# 1. Browser AI runtime (transformers.js + ONNX Runtime Web), fetched with `npm pack`.
FROM node:22.22.0-bookworm-slim AS vendor
WORKDIR /src
COPY scripts/vendor.sh scripts/
RUN --mount=type=secret,id=extra_ca,required=false \
    if [ -f /run/secrets/extra_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/extra_ca; fi; \
    mkdir web && sh scripts/vendor.sh

# 2. Static Go binary with the web UI and runtime embedded.
FROM golang:1.24.7-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=extra_ca,required=false \
    if [ -f /run/secrets/extra_ca ]; then cat /etc/ssl/certs/ca-certificates.crt /run/secrets/extra_ca > /tmp/ca.pem; export SSL_CERT_FILE=/tmp/ca.pem; fi; \
    go mod download
COPY *.go ./
COPY migrations migrations
COPY web web
COPY --from=vendor /src/web/vendor web/vendor
RUN go vet ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/photoly . \
    && mkdir -p /out/data /out/tmp

# 3. Runtime: no shell, no package manager; runs as a non-root user.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/photoly /photoly
# Large uploads spill to /tmp; /data holds photos and the downloaded model.
COPY --from=build --chown=65532:65532 --chmod=1777 /out/tmp /tmp
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV PHOTOLY_DATA=/data PHOTOLY_ADDR=:8084
EXPOSE 8084
VOLUME /data
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=3 CMD ["/photoly", "health"]
ENTRYPOINT ["/photoly"]
CMD ["serve"]
