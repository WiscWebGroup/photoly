"""Measure real CPU OCR on the bundled text/photograph fixtures (no database needed)."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import psutil


def main():
    fixtures = Path(__file__).resolve().parents[1] / 'tests' / 'fixtures'
    paths = sorted((fixtures / 'ocr').glob('*.png')) + [fixtures / 'ai' / 'rocket.jpg']
    reports = []
    for source in paths:
        with tempfile.TemporaryDirectory(prefix='photoly-ocr-smoke-') as work:
            start = time.monotonic()
            process = subprocess.Popen([sys.executable, '-m', 'app.ocr.worker', str(source), work],
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
            parent = psutil.Process(process.pid)
            peak = 0
            while process.poll() is None:
                if time.monotonic() - start >= 45:
                    os.killpg(process.pid, signal.SIGKILL)
                    break
                try:
                    total = 0
                    for child in [parent, *parent.children(recursive=True)]:
                        try:
                            total += child.memory_info().rss
                        except psutil.NoSuchProcess:
                            pass
                    peak = max(peak, total)
                except psutil.NoSuchProcess:
                    pass
                time.sleep(0.01)
            stdout, _ = process.communicate()
            result = json.loads(stdout) if process.returncode == 0 else {'error': 'worker_failed_or_timed_out'}
            reports.append({'fixture': source.name, 'seconds': round(time.monotonic() - start, 3),
                            'peak_process_tree_rss_mib': round(peak / 1048576, 1), 'result': result})
    print(json.dumps(reports, ensure_ascii=False, indent=2))
    return int(any('error' in row['result'] for row in reports))


if __name__ == '__main__':
    sys.exit(main())
