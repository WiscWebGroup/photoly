import os

bind = "0.0.0.0:8084"
workers = int(os.getenv("GUNICORN_WORKERS", "2"))
threads = int(os.getenv("GUNICORN_THREADS", "4"))
worker_class = "gthread"
timeout = int(os.getenv("GUNICORN_TIMEOUT", "120"))
graceful_timeout = 30
accesslog = "-"
errorlog = "-"
pidfile = os.getenv("GUNICORN_PID_FILE", "/tmp/photoly-gunicorn.pid")
