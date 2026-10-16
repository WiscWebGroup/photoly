FROM python:3.12-slim AS base
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy PATH="/app/.venv/bin:$PATH" GUNICORN_PID_FILE=/tmp/photoly-gunicorn.pid UPLOAD_FOLDER=/photoly
RUN apt-get update && apt-get install -y --no-install-recommends libimage-exiftool-perl gosu \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir uv==0.13.0 \
    && groupadd --gid 10001 photoly && useradd --uid 10001 --gid photoly --no-create-home photoly
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev
FROM base AS application
COPY app ./app
COPY migrations ./migrations
COPY alembic.ini ./
COPY wsgi.py gunicorn.conf.py entrypoint.sh migrate.sh ./
RUN chmod +x entrypoint.sh migrate.sh
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8084/healthz', timeout=4)"
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["gunicorn", "--config", "gunicorn.conf.py", "wsgi:app"]

FROM application AS test
RUN uv sync --frozen
COPY tests ./tests
CMD ["pytest", "-q"]

FROM application AS production

FROM base AS inference
ARG INFERENCE_VARIANT=cuda
COPY inference/${INFERENCE_VARIANT}/ /app/inference/
RUN --mount=type=cache,target=/root/.cache/uv UV_PROJECT_ENVIRONMENT=/app/.venv uv sync --project /app/inference --frozen --inexact
COPY app ./app
ENV HF_HOME=/models HF_HUB_DISABLE_TELEMETRY=1 TOKENIZERS_PARALLELISM=false PYTORCH_KERNEL_CACHE_PATH=/tmp/torch-kernels
RUN mkdir -p /models && chown photoly:photoly /models
USER photoly
ENTRYPOINT ["python", "-m", "app.ai.runtime"]
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s CMD python -c "import os,urllib.request; urllib.request.urlopen(urllib.request.Request('http://localhost:8090/status',headers={'Authorization':'Bearer '+os.environ['AI_INFERENCE_TOKEN']}),timeout=4)"
