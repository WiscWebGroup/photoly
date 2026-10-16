#!/bin/sh
set -eu
backend_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
project_dir="$backend_dir/inference/mps"
case "${1:-}" in
  install)
    [ "$(uname -s)" = Darwin ] && [ "$(uname -m)" = arm64 ] || {
      echo 'Requires a native Apple Silicon macOS shell.' >&2; exit 1;
    }
    [ "$(sw_vers -productVersion | cut -d . -f 1)" -ge 14 ] || {
      echo 'Requires macOS 14 or later for the locked native dependencies.' >&2; exit 1;
    }
    exec uv sync --project "$project_dir" --frozen --python 3.12
    ;;
  check|start)
    [ -x "$project_dir/.venv/bin/python" ] || {
      echo "Run sh $0 install first." >&2; exit 1;
    }
    exec "$project_dir/.venv/bin/python" "$backend_dir/scripts/mps_runtime.py" "$@"
    ;;
  *) echo "Usage: sh $0 install|check|start [--env-file PATH]" >&2; exit 2 ;;
esac
