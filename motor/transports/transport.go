package transports

import (
	"regexp"

	"github.com/spf13/afero"
	"go.mondoo.io/mondoo/nexus/assets"
)

type Transport interface {
	// RunCommand executes a command on the target system
	RunCommand(command string) (*Command, error)
	// returns file permissions and ownership
	FileInfo(path string) (FileInfoDetails, error)
	// FS provides access to the file system of the target system
	FS() afero.Fs
	// Close closes the transport
	Close()
	// returns if this is a static asset that does not allow run command
	Capabilities() Capabilities

	Kind() assets.Kind
	Runtime() string
}

type FileSearch interface {
	Find(from string, r *regexp.Regexp, typ string) ([]string, error)
}