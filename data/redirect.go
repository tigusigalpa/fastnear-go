package data

import (
	"errors"
	"net/url"
	"strconv"
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
		return 0, errors.New("missing Location header")
	}
	u, err := url.Parse(location)
	if err != nil {
		return 0, errors.New("invalid Location URL")
	}
	location = strings.TrimSuffix(u.Path, "/")
	var prefix string
	for _, p := range []string{"/v0/block/", "/v0/block_opt/"} {
		if strings.HasPrefix(location, p) {
			prefix = p
			break
		}
	}
	if prefix == "" {
		return 0, errors.New("location does not point at a block")
	}
	heightStr := location[len(prefix):]
	h, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return 0, errors.New("location has an invalid or overflowing block height")
	}
	return h, nil
}
