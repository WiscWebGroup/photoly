---
title: Connection status and cached browsing
---

Remote SMB, WebDAV and S3 mount folders show an unavailable badge when an access check fails. Inside the mount or a descendant folder, an orange banner explains the failure and offers **Check again**. Indexed folders, cached thumbnails and available previews remain browsable. Notes are stored in Photoly and remain available independently of the remote file.

Connection checks run from **the Photoly server**, not from the browser. A phone outside your home network can still use a NAS that the Photoly server can reach. Conversely, a browser connected to Wi-Fi does not make a NAS accessible to a Photoly server without a route to it.

Checks distinguish connection failures, authentication or permission failures, invalid configuration and disabled sources. A single missing file does not mark the entire source disconnected. Scan errors remain separate: an unsupported or corrupt image does not imply a network failure. The viewer reports storage/file access failures instead of a generic EXIF error; a photo without EXIF is normal.

The root checks visible remote mounts; a remote folder checks its own source. Checks refresh every 30 seconds while the page is visible, resume when it becomes visible, and have a five-second server deadline. Concurrent checks for the same configuration share one request. **Check again** bypasses the cached result. Configuration changes invalidate the cache. Actual remote fetches can update status, but loading a cached file does not prove the source has reconnected.

Health checks are read-only:

- SMB/WebDAV query the configured root directory.
- S3 checks an existing original object's metadata; an empty source uses a bucket HEAD. Permissions for bucket metadata can differ from permissions for individual objects, so an access-denied result on an empty source should be checked against its IAM policy.
- The checks never call the existing write-test routine or create/delete probe files.

If the banner persists, check the source address, server-side network/VPN/DNS/TLS, credentials and root/share/bucket permissions. A configured host that is not allowed by Photoly's storage allowlist is a configuration problem. Disconnecting storage does not bypass normal authorization or make failed file writes succeed.

If the browser itself cannot reach Photoly, the UI reports an application connection error, not a NAS outage. Source APIs accept only account-owned source IDs and never expose credentials, internal paths or raw protocol errors. Health state is ephemeral and is rechecked after a server restart. In a multi-process deployment, each process maintains its own short-lived cache.
