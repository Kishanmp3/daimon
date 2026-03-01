//go:build !prod

package breaklog

import "io/fs"

// WebDistFS returns nil in dev builds.
// The server falls back to finding web/dist on disk.
func WebDistFS() (fs.FS, error) {
	return nil, nil
}
