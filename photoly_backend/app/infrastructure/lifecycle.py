import os
import signal
from pathlib import Path
from threading import Timer


class ServerLifecycle:
    """Only signal the Gunicorn master registered by our deployment, never a guessed PID."""
    def __init__(self, pid_file=None):
        self.pid_file = pid_file

    def stop(self):
        if not self.pid_file:
            return -1
        try:
            pid = int(Path(self.pid_file).read_text().strip())
            if pid != os.getppid():
                return -1
            os.kill(pid, 0)
        except (OSError, ValueError):
            return -1
        timer = Timer(0.5, os.kill, args=(pid, signal.SIGTERM))
        timer.daemon = True
        timer.start()
        return 1
