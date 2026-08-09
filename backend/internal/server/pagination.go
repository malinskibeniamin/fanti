package server

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
)

// Page tokens are opaque base64 offsets. Keyset pagination can replace
// this without changing the API surface.

func encodePageToken(offset int) string {
	return base64.URLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}

	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid page token: %w", err))
	}

	offset, err := strconv.Atoi(string(raw))
	if err != nil || offset < 0 {
		return 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid page token %q", token)) //nolint:err113 // request detail
	}

	return offset, nil
}
