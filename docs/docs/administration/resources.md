---
title: Resource monitoring
---

Admin → Overview → Resource shows host CPU and Photoly process memory. Linux
production deployments sample every 10 seconds independently of AI enablement,
maintenance and scheduling pause. On Docker, host CPU is the Linux host/VM
visible through `/proc/stat`, **not** the container quota percentage. Unsupported
platforms report unavailable measurements, never synthetic idle values.

Memory is resident bytes (RSS) for the backend process, including its in-process
Scheduler, plus the separate AI worker. It excludes Swap, PostgreSQL, nginx and
temporary media-tool child processes. The ring shows their relative shares of
the summed process RSS; shared OS pages can be accounted to multiple processes.
Reading the AI measurement does not start a worker or load a model.

The page refreshes current measurements every 30 seconds and history every 10
minutes while visible. History persists 10-minute means and peaks in PostgreSQL,
retains 48 hours, and offers 12/24-hour views. Current partial intervals are shown
immediately. Missing measurements are excluded from aggregates; sample counts
are available in History data. Distinct process boots and missing intervals are
not joined across a history curve. An abrupt crash can lose the current,
uncommitted interval; completed intervals survive restarts. Database outages
buffer at most 48 hours in memory and do not stop media operations.

Backend Uptime measures the current service process, rather than host boot or AI
worker lifetime. It resets on deployment/restart and stops extrapolating after a
failed refresh. The frontend uses a monotonic local elapsed clock between server
calibrations.

Admin-session endpoints:

- `GET /admin/runtime`: start time, uptime seconds, boot ID.
- `GET /admin/resources/current`: sample time, scope, nullable CPU/RSS values.
- `GET /admin/resources/history?hours=12|24`: completed and current intervals.

The initial implementation monitors one backend instance. It does not aggregate
multiple replicas. Migration 16 creates a small `resource_history` table; no
historical data is fabricated during upgrade. Back up the database before
migration and allow at least two samples for the first CPU measurement.
