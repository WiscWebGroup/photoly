#!/bin/sh
# Downloads the browser inference runtime into web/vendor/ (gzipped) so the Go binary can
# embed it. Uses `npm pack`, so no dependencies or install scripts run.
set -eu
TRANSFORMERS_VERSION=4.3.1
# Must match the onnxruntime-web version that transformers.js pins.
ORT_WEB_VERSION=1.31.0-dev.20260914-8d85527a0

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/web/vendor"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cd "$work"
npm pack --silent "@huggingface/transformers@$TRANSFORMERS_VERSION" "onnxruntime-web@$ORT_WEB_VERSION" >/dev/null
mkdir t o
tar -xzf huggingface-transformers-*.tgz -C t
tar -xzf onnxruntime-web-*.tgz -C o

rm -rf "$out" && mkdir -p "$out"
cp t/package/dist/transformers.min.js "$out/"
cp o/package/dist/ort-wasm-simd-threaded.asyncify.mjs o/package/dist/ort-wasm-simd-threaded.asyncify.wasm "$out/"
cp t/package/LICENSE "$out/LICENSE.transformers.txt"
# The onnxruntime-web package ships no LICENSE file; it is MIT-licensed.
printf 'onnxruntime-web %s\nMIT License, Copyright (c) Microsoft Corporation.\nhttps://github.com/microsoft/onnxruntime/blob/main/LICENSE\n' "$ORT_WEB_VERSION" > "$out/LICENSE.onnxruntime-web.txt"
gzip -9 -n "$out"/*.js "$out"/*.mjs "$out"/*.wasm
ls -l "$out"
