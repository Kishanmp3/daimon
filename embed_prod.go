//go:build prod

package breaklog

import (
	"embed"
	"io/fs"
)

//go:embed web/dist
var webDist embed.FS

// WebDistFS returns the embedded web dashboard filesystem.
// Only available in binaries built with -tags prod.
func WebDistFS() (fs.FS, error) {
	return fs.Sub(webDist, "web/dist")
}
