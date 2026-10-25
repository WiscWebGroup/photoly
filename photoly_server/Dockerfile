# syntax=docker/dockerfile:1
# Single-binary Photoly backend: `photoly serve|migrate|ai-worker|verify`.
# Build context: photoly_server/. Runtime is debian-slim: ONNX Runtime plus the media CLIs (ffmpeg, heif-convert, exiftool, tesseract).

FROM golang:1.24.7-bookworm AS build
ARG TARGETARCH
# Must match the C API version onnxruntime_go (go.mod) was written for.
ARG ORT_VERSION=1.29.0
RUN case "$TARGETARCH" in \
      amd64) arch=x64 sum=c3fddc4f139a045b0c4902c57410f0694f1c2fdf9b6939fbe38b1aeae7cd14ba ;; \
      arm64) arch=aarch64 sum=e1799098ebc054b370f6176a450f158720f297818c613e5dc99b92e2ec82346f ;; \
      *) echo "no ONNX Runtime build for $TARGETARCH" >&2; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/ort.tgz "https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION/onnxruntime-linux-$arch-$ORT_VERSION.tgz" \
    && echo "$sum  /tmp/ort.tgz" | sha256sum -c - \
    && tar -xzf /tmp/ort.tgz -C /tmp && ort=/tmp/onnxruntime-linux-$arch-$ORT_VERSION \
    && mkdir -p /out/lib /out/data /out/tmp \
    && cp -L $ort/lib/libonnxruntime.so.1 /out/lib/ \
    && cp $ort/LICENSE /out/lib/LICENSE.onnxruntime.txt \
    && rm -rf /tmp/ort.tgz $ort
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... \
    && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/photoly-bin ./cmd/photoly

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 ffmpeg libheif-examples \
      libimage-exiftool-perl tesseract-ocr tesseract-ocr-eng tesseract-ocr-chi-sim tesseract-ocr-chi-tra tesseract-ocr-jpn \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -g 65532 nonroot && useradd -u 65532 -g 65532 -M -s /usr/sbin/nologin nonroot
COPY --from=build /out/lib /usr/local/lib/onnxruntime
COPY --from=build /out/photoly-bin /usr/local/bin/photoly
COPY --from=build --chown=65532:65532 --chmod=1777 /out/tmp /tmp
# Uploaded media lives in UPLOAD_FOLDER (mount ./upload_data here, owned by uid 65532).
COPY --from=build --chown=65532:65532 /out/data /photoly
# Downloaded model weights; mount a volume here to keep them across rebuilds.
COPY --from=build --chown=65532:65532 /out/data /var/lib/photoly
USER 65532:65532
ENV PHOTOLY_ADDR=:8084 UPLOAD_FOLDER=/photoly PHOTOLY_DATA=/var/lib/photoly ONNXRUNTIME_LIB=/usr/local/lib/onnxruntime/libonnxruntime.so.1 ORT_DISABLE_TELEMETRY=1
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 CMD ["/usr/local/bin/photoly", "health"]
ENTRYPOINT ["/usr/local/bin/photoly"]
CMD ["serve"]
