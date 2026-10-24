# syntax=docker/dockerfile:1
#
# Builds the POC image: the binary, the ONNX Runtime CPU library for optional server-side
# inference, and a distroless base with glibc and libstdc++ (which ONNX Runtime needs).
# Behind a TLS-intercepting proxy, pass its CA so npm, curl and Go can download:
#   docker build --secret id=extra_ca,src=/path/to/proxy-ca.crt .

# 1. Browser AI runtime (transformers.js + ONNX Runtime Web), fetched with `npm pack`.
FROM node:22.22.0-bookworm-slim AS vendor
WORKDIR /src
COPY scripts/vendor.sh scripts/
RUN --mount=type=secret,id=extra_ca,required=false \
    if [ -f /run/secrets/extra_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/extra_ca; fi; \
    mkdir web && sh scripts/vendor.sh

# 2. Go binary with the web UI and browser runtime embedded, plus ONNX Runtime. The binary
#    uses cgo only to load ONNX Runtime at startup; CGO_ENABLED=0 still builds a static
#    binary without server inference.
FROM golang:1.24.7-bookworm AS build
ARG TARGETARCH
# Must match the C API version onnxruntime_go (go.mod) was written for.
ARG ORT_VERSION=1.29.0
RUN --mount=type=secret,id=extra_ca,required=false \
    if [ -f /run/secrets/extra_ca ]; then cat /etc/ssl/certs/ca-certificates.crt /run/secrets/extra_ca > /tmp/ca.pem; export CURL_CA_BUNDLE=/tmp/ca.pem; fi; \
    case "$TARGETARCH" in \
      amd64) arch=x64 sum=c3fddc4f139a045b0c4902c57410f0694f1c2fdf9b6939fbe38b1aeae7cd14ba ;; \
      arm64) arch=aarch64 sum=e1799098ebc054b370f6176a450f158720f297818c613e5dc99b92e2ec82346f ;; \
      *) echo "no ONNX Runtime build for $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/ort.tgz "https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION/onnxruntime-linux-$arch-$ORT_VERSION.tgz" \
    && echo "$sum  /tmp/ort.tgz" | sha256sum -c - \
    && tar -xzf /tmp/ort.tgz -C /tmp && ort=/tmp/onnxruntime-linux-$arch-$ORT_VERSION \
    && mkdir -p /out/lib \
    && cp -L $ort/lib/libonnxruntime.so.1 /out/lib/ \
    && cp $ort/LICENSE /out/lib/LICENSE.onnxruntime.txt \
    && cp $ort/ThirdPartyNotices.txt /out/lib/ThirdPartyNotices.onnxruntime.txt \
    && rm -rf /tmp/ort.tgz $ort
ENV ONNXRUNTIME_LIB=/out/lib/libonnxruntime.so.1
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
    && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/photoly . \
    && mkdir -p /out/data /out/tmp

# 3. Runtime: no shell, no package manager; runs as a non-root user.
FROM gcr.io/distroless/cc-debian12:nonroot@sha256:9dac0a79194e45a7da0158a9c6da57b217585af0786db3845d1f0ec1a0dd182f
COPY --from=build /out/lib /usr/local/lib/onnxruntime
COPY --from=build /out/photoly /photoly
# Large uploads spill to /tmp; /data holds photos and the downloaded model.
COPY --from=build --chown=65532:65532 --chmod=1777 /out/tmp /tmp
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV PHOTOLY_DATA=/data PHOTOLY_ADDR=:8084 ONNXRUNTIME_LIB=/usr/local/lib/onnxruntime/libonnxruntime.so.1
EXPOSE 8084
VOLUME /data
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=3 CMD ["/photoly", "health"]
ENTRYPOINT ["/photoly"]
CMD ["serve"]
