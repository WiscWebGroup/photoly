// Photoly additions to pinned upstream: guarded handles, no-follow opens and
// handle-based no-replace rename. Kept separate to make upstream review easy.
package smb2

import (
	"github.com/cloudsoda/go-smb2/internal/smb2"
	"os"
)

// OpenGuarded refuses symlinks and denies concurrent write/delete access. The
// caller holds ancestor directory handles until the operation completes.
// mode: read, guard (attributes only), write (existing), create, directory, mutate.
func (s *Share) OpenGuarded(name, mode string) (*File, error) {
	name = normPath(name)
	if name != "" {
		if e := validatePath("guarded", name, false); e != nil {
			return nil, e
		}
	}
	access := uint32(smb2.FILE_READ_ATTRIBUTES)
	disposition := uint32(smb2.FILE_OPEN)
	options := uint32(smb2.FILE_OPEN_REPARSE_POINT)
	switch mode {
	case "read":
		access |= smb2.FILE_READ_DATA
	case "guard":
		options |= smb2.FILE_DIRECTORY_FILE
	case "write":
		access |= smb2.FILE_WRITE_DATA
	case "create":
		access |= smb2.FILE_WRITE_DATA | smb2.DELETE
		disposition = smb2.FILE_CREATE
		options |= smb2.FILE_NON_DIRECTORY_FILE
	case "directory":
		disposition = smb2.FILE_CREATE
		options |= smb2.FILE_DIRECTORY_FILE
	case "mutate":
		access |= smb2.DELETE
	default:
		return nil, os.ErrInvalid
	}
	f, e := s.createFile(name, &smb2.CreateRequest{ImpersonationLevel: smb2.Impersonation, DesiredAccess: access, ShareAccess: smb2.FILE_SHARE_READ, CreateDisposition: disposition, CreateOptions: options, Mapping: s.mapping}, false)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	if info.(*FileStat).FileAttributes&smb2.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		f.Close()
		return nil, os.ErrPermission
	}
	return f, nil
}
func (f *File) RenameNoReplace(destination string) error {
	destination = normPath(destination)
	if e := validatePath("rename", destination, false); e != nil {
		return e
	}
	return f.setInfo(&smb2.SetInfoRequest{FileInfoClass: smb2.FileRenameInformation, Input: &smb2.FileRenameInformationType2Encoder{ReplaceIfExists: 0, FileName: destination, Mapping: f.mapping}})
}
func (f *File) RemoveGuarded() error { return f.remove() }
