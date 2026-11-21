package data

import (
	"math"
	"testing"
)

func TestLocationHeightBoundaries(t *testing.T) {
	for _, input := range []string{"/v0/block/18446744073709551616", "/v0/block/9999999999999999999999999", "/v0/block/12junk"} {
		if _, err := parseLocationHeight(input); err == nil {
			t.Fatalf("invalid height accepted: %s", input)
		}
	}
	h, err := parseLocationHeight("https://mirror.test/v0/block/18446744073709551615/?apiKey=ignored#part")
	if err != nil || h != math.MaxUint64 {
		t.Fatalf("max height = %d %v", h, err)
	}
}

func TestTimestampParsingIsExact(t *testing.T) {
	for _, input := range []string{"123junk", "123456789012345678901", ""} {
		h := BlockHeader{TimestampNanosec: input, Timestamp: 456}
		got, err := h.Time()
		if err != nil || got.UnixNano() != 456 {
			t.Fatalf("malformed timestamp %q: %v %v", input, got, err)
		}
	}
	h := BlockHeader{TimestampNanosec: "0", Timestamp: 456}
	got, _ := h.Time()
	if got.UnixNano() != 0 {
		t.Fatal("valid zero timestamp replaced by fallback")
	}
}
