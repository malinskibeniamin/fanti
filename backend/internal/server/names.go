// Package server implements the fanti.v1 Connect services.
package server

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
)

// parseName strips a "collection/" prefix from an AIP resource name and
// returns the id, or an INVALID_ARGUMENT error.
func parseName(collection, name string) (string, error) {
	id, ok := strings.CutPrefix(name, collection+"/")
	if !ok || id == "" || strings.Contains(id, "/") {
		return "", connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must be %s/{id}, got %q", collection, name)) //nolint:err113 // request detail
	}

	return id, nil
}
