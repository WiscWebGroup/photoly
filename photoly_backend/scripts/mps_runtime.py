"""Native macOS launcher. Load only runtime settings; never execute a dotenv file."""
import argparse
import os
from pathlib import Path
import platform
import sys

BACKEND = Path(__file__).resolve().parents[1]
KEYS = ("AI_INFERENCE_TOKEN", "AI_CONTROL_TOKEN", "AI_RUNTIME_HOST", "AI_RUNTIME_PORT", "HF_HOME")


def configure(env_file):
    from dotenv import dotenv_values
    values = dotenv_values(env_file, interpolate=False) if env_file.exists() else {}
    for key in KEYS:
        if key not in os.environ and values.get(key) is not None:
            os.environ[key] = values[key]
    for key in ("AI_INFERENCE_TOKEN", "AI_CONTROL_TOKEN"):
        if not os.environ.get(key, "").strip():
            raise SystemExit(f"Set {key} in the environment or {env_file}")
    os.environ.setdefault("AI_RUNTIME_HOST", "0.0.0.0")
    os.environ.setdefault("AI_RUNTIME_PORT", "8090")
    try:
        port = int(os.environ["AI_RUNTIME_PORT"])
        if not 1 <= port <= 65535:
            raise ValueError()
    except ValueError:
        raise SystemExit("AI_RUNTIME_PORT must be between 1 and 65535") from None
    os.environ.setdefault("HF_HOME", str(Path.home() / ".cache/photoly/models"))
    os.environ["HF_HOME"] = str(Path(os.environ["HF_HOME"]).expanduser().resolve())
    os.environ["HF_HUB_DISABLE_TELEMETRY"] = "1"
    os.environ["TOKENIZERS_PARALLELISM"] = "false"
    os.environ["PYTORCH_ENABLE_MPS_FALLBACK"] = "0"
    os.environ.pop("AI_TEST_RUNTIME", None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("check", "start"))
    parser.add_argument("--env-file", type=Path, default=BACKEND.parent / ".env")
    args = parser.parse_args()
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise SystemExit("Use native ARM64 Python on an Apple Silicon Mac (not Rosetta or Docker).")
    if int(platform.mac_ver()[0].split(".")[0]) < 14:
        raise SystemExit("The locked native dependencies require macOS 14 or later.")
    configure(args.env_file)
    sys.path.insert(0, str(BACKEND))
    os.chdir(BACKEND)
    from app.ai.devices import probe_hardware
    devices = probe_hardware()
    if not next(item["available"] for item in devices if item["device"] == "mps"):
        raise SystemExit("MPS is unavailable. Check macOS and the native ARM64 PyTorch installation.")
    import torch
    # Exercise a real Metal allocation/kernel, without downloading model weights.
    if (torch.ones(1, device="mps") + 1).cpu().item() != 2:
        raise SystemExit("MPS kernel verification failed.")
    torch.mps.empty_cache()
    print(f"MPS ready: PyTorch {torch.__version__}; cache: {os.environ['HF_HOME']}", flush=True)
    if args.command == "start":
        from app.ai.runtime import serve
        serve()


if __name__ == "__main__":
    main()
