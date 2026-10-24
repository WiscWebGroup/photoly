#!/bin/sh
# ONNX (Go worker, in Docker with libonnxruntime) vs PyTorch SigLIP2 vector parity.
# usage: scripts/onnx_vs_pytorch.sh WORKDIR   (from anywhere; needs docker, the HF cache and the mps .venv)
# Steps: build the Dockerfile's build stage, run a throwaway Go test inside it that dumps vectors,
# then compare with PyTorch (photoly_backend/app/ai profile: Pillow + HF processor, fp32, CPU).
set -eu
W=$(mkdir -p "$1" && cd "$1" && pwd)
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
HF=${HF_HOME:-$HOME/.cache/huggingface}/hub
ONNX_SNAP=$(ls -d "$HF"/models--onnx-community--siglip2-base-patch16-224-ONNX/snapshots/ba1f3b0843f24bc5417d38e19c37b287d719b2f4)
PY=${PYTHON:-$ROOT/photoly_backend/inference/mps/.venv/bin/python}

# model layout the Go cache expects: <dir>/<revision>/{tokenizer.json,onnx/*}
M=$W/models/ba1f3b0843f24bc5417d38e19c37b287d719b2f4; mkdir -p "$M/onnx"
for f in tokenizer.json onnx/vision_model.onnx onnx/text_model.onnx; do [ -f "$M/$f" ] || cp -L "$ONNX_SNAP/$f" "$M/$f"; done

"$PY" -I "$ROOT/photoly_server/scripts/onnx_vs_pytorch.py" prepare "$ROOT" "$W"
docker build -q --target build -t photoly-onnx-build -f "$ROOT/photoly_server/Dockerfile" "$ROOT/photoly_server" >/dev/null
docker run --rm -v "$ROOT:/repo:ro" -v "$W:/w" -e PHOTOLY_TEST_MODELS=/w/models -e DUMP_IN=/w/request.json -e DUMP_OUT=/w/onnx.json \
  -e ONNXRUNTIME_LIB=/out/lib/libonnxruntime.so.1 -e ORT_DISABLE_TELEMETRY=1 photoly-onnx-build sh -c \
  'cp /repo/photoly_server/scripts/onnx_dump_test.go.txt /src/internal/ai/zz_onnx_dump_test.go && cd /src && CGO_ENABLED=1 go test ./internal/ai -run TestDumpVectors -count=1 -v 2>&1 | tail -5'
HF_HUB_OFFLINE=1 "$PY" -I "$ROOT/photoly_server/scripts/onnx_vs_pytorch.py" compare "$ROOT" "$W"
