FROM python:3.12-slim AS base
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy PATH="/app/.venv/bin:$PATH" GUNICORN_PID_FILE=/tmp/photoly-gunicorn.pid UPLOAD_FOLDER=/photoly
RUN apt-get update && apt-get install -y --no-install-recommends libimage-exiftool-perl gosu \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir uv==0.13.0 \
    && groupadd --gid 10001 photoly && useradd --uid 10001 --gid photoly --no-create-home photoly
WORKDIR /app
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev
COPY app ./app
COPY wsgi.py gunicorn.conf.py entrypoint.sh ./
RUN chmod +x entrypoint.sh
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8084/healthz', timeout=4)"
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["gunicorn", "--config", "gunicorn.conf.py", "wsgi:app"]

FROM base AS test
RUN uv sync --frozen
COPY tests ./tests
CMD ["pytest", "-q"]

FROM base AS production
