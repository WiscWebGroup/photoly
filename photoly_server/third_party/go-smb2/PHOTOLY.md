# Pinned SMB client

Upstream: https://github.com/CloudSoda/go-smb2
Revision: 95daac2ef783 (v0.0.0-20261029174947-95daac2ef783).
The upstream BSD-2-Clause license is preserved in LICENSE. Upstream tests and CI
configuration are excluded from this source snapshot.

Photoly additions are isolated in `photoly.go`: open a non-following handle with
server-enforced deny-write/deny-delete sharing, rename that handle with
ReplaceIfExists=false, and delete through the validated handle. The storage
adapter pins every ancestor, refuses all reparse points, and checks the content
version after opening the exclusive mutation handle. The standard upstream
Rename intentionally replaces existing destinations and is not used by Photoly.

When updating, review these methods and run the Samba and real NAS integration
tests, including stale writes, destination conflicts and link replacement.

`internal/crypto/ccm/ccm.go` checks payload plus authentication-block length before
addition, and `util.go` checks append lengths before integer addition,
rejecting negative or overflowing sizes before slicing or allocating. Keep this
guard when updating the snapshot. Run `go test -race ./internal/crypto/...` from
this module; the parent module's `go test ./...` does not include replacement
module tests.
