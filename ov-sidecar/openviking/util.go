package openviking

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

// ErrPathTraversal is returned when a URI resolves outside the allowed prefix.
var ErrPathTraversal = errors.New("path traversal denied")

const vikingScheme = "viking://"

// ResolveURI converts a viking:// URI to a local filesystem path and validates
// it stays within cfg.OpenVikingPrefix (path traversal check).
func ResolveURI(cfg *config.Config, uri string) (string, error) {
	cleanURI := strings.TrimPrefix(uri, vikingScheme)
	cleanURI = strings.TrimPrefix(cleanURI, "/")

	fullPath := filepath.Join(cfg.OpenVikingPrefix, cleanURI)

	rel, err := filepath.Rel(cfg.OpenVikingPrefix, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrPathTraversal
	}
	return fullPath, nil
}

// ValidateURIs checks all URIs for path traversal before reading any files.
// Returns error on first traversal violation.
func ValidateURIs(cfg *config.Config, uris []string) error {
	for _, uri := range uris {
		if _, err := ResolveURI(cfg, uri); err != nil {
			return err
		}
	}
	return nil
}
