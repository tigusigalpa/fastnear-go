package data

import (
	"fmt"
	"strings"
)

// parseLocationHeight extracts the block height from a NEAR Data
// redirect Location. Supported forms:
//
//	/v0/block/{height}          (final blocks)
//	/v0/block_opt/{height}      (optimistic blocks)
//
// Absolute URLs with the same suffix are also accepted.
func parseLocationHeight(location string) (uint64, error) {
	if location == "" {
		return 0, fmt.Errorf("missing Location header")
	}
	// Strip scheme://host if the Location is absolute.
	if i := strings.Index(location, "://"); i >= 0 {
		rest := location[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			location = rest[j:]
		} else {
			return 0, fmt.Errorf("Location %q has no path", location)
		}
	}
	location = strings.TrimSuffix(location, "/")
	var prefix string
	for _, p := range []string{"/v0/block/", "/v0/block_opt/"} {
		if strings.HasPrefix(location, p) {
			prefix = p
			break
		}
	}
	if prefix == "" {
		return 0, fmt.Errorf("Location %q does not point at a block", location)
	}
	heightStr := location[len(prefix):]
	if i := strings.IndexByte(heightStr, '?'); i >= 0 {
		heightStr = heightStr[:i]
	}
	var h uint64
	if heightStr == "" {
		return 0, fmt.Errorf("Location %q has no height", location)
	}
	for i := 0; i < len(heightStr); i++ {
		c := heightStr[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("Location %q has a non-numeric height", location)
		}
		h = h*10 + uint64(c-'0')
	}
	return h, nil
}
