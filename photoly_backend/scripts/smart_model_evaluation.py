"""Opt-in fixture evaluation; the Smart scheduler itself never calls a model."""
import argparse
import hashlib
import json
from pathlib import Path
import resource
import sys
import time
import numpy as np
import torch
from app.ai.registry import adapter, profile_spec
from app.ai.types import RuntimeConfig
from app.ai.preprocessing import decode_image

parser = argparse.ArgumentParser()
parser.add_argument('provider', choices=['siglip2', 'qwen3_vl'])
parser.add_argument('--output', required=True, type=Path)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1] / 'tests/fixtures/ai'
manifest = json.loads((root / 'manifest.json').read_text())
spec = profile_spec(args.provider)
model = adapter(args.provider)
report = {'provider': args.provider, 'dimension': spec['dimension'], 'fingerprint': spec['fingerprint'], 'device': 'mps',
    'precision': 'float32', 'role': 'document', 'threshold': .80, 'margin': .05,
    'scope': 'Four fixed public photos and 5% inset crops, not a labeled travel dataset.'}
started = time.perf_counter()
try:
    model.load(spec, RuntimeConfig('mps', 'float32', 4))
    report['loadSeconds'] = time.perf_counter() - started
    originals, crops, times = [], [], []
    for sample in manifest['samples']:
        data = (root / sample['file']).read_bytes()
        assert hashlib.sha256(data).hexdigest() == sample['sha256']
        image = decode_image(data)
        width, height = image.size
        crop = image.crop((round(width*.05), round(height*.05), round(width*.95), round(height*.95)))
        for target, picture in ((originals, image), (crops, crop)):
            start = time.perf_counter()
            target.append(model.encode_images([picture], role='document').vectors[0])
            times.append(time.perf_counter() - start)
    matrix = np.array(crops) @ np.array(originals).T
    results = []
    for i, row in enumerate(matrix):
        order = np.argsort(-row)
        results.append({'file': manifest['samples'][i]['file'], 'correctScore': float(row[i]),
            'top1': manifest['samples'][int(order[0])]['file'], 'margin': float(row[order[0]]-row[order[1]]),
            'accepted': bool(row[order[0]] >= .8 and row[order[0]] - row[order[1]] >= .05)})
    report.update(results=results, cosineMatrix=matrix.tolist(), inferenceMedianSeconds=float(np.median(times)),
        mpsAllocatedBytes=torch.mps.current_allocated_memory(), mpsDriverBytes=torch.mps.driver_allocated_memory(),
        peakRssBytes=resource.getrusage(resource.RUSAGE_SELF).ru_maxrss * (1 if sys.platform == 'darwin' else 1024))
except Exception as error:
    report['error'] = type(error).__name__ + ': ' + str(error)[:500]
finally:
    model.close()
    report['afterUnloadBytes'] = torch.mps.current_allocated_memory()
    report['totalSeconds'] = time.perf_counter() - started
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report), flush=True)
