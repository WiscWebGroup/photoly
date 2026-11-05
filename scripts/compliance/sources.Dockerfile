# Collect corresponding sources from an exact runtime image in an isolated build.
ARG RUNTIME_IMAGE=debian:bookworm-slim
FROM ${RUNTIME_IMAGE} AS collect
USER root
# Capture the runtime graph before adding Python (the collector reads this file).
RUN dpkg-query -W -f='${db:Status-Status}\t${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\n' | awk -F '\t' '$1 == "installed" {sub(/^[^\t]*\t/, ""); print}' > /tmp/runtime-packages.tsv \
    && apt-get update && apt-get install -y --no-install-recommends python3 ca-certificates
COPY collect-debian-sources.py /collect.py
COPY collect-javascript-sources.py /collect-javascript.py
RUN --mount=type=cache,target=/source-cache,sharing=locked python3 /collect.py && python3 /collect-javascript.py
FROM scratch
COPY --from=collect /compliance-sources /system
COPY --from=collect /usr/share/photoly/licenses /notices
COPY --from=collect /usr/share/photoly/sources /dependencies
