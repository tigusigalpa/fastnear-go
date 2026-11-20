package transport

import "testing"

func TestEncodeSegment(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"graph/follow/sleet.near", "graph%2Ffollow%2Fsleet.near"},
		{"ed25519:ABCdef123", "ed25519:ABCdef123"},
		{"secp256k1:xyz", "secp256k1:xyz"},
		{"ml-dsa-65-hash:Ab1", "ml-dsa-65-hash:Ab1"},
		{"root.near", "root.near"},
		{"with space", "with%20space"},
		{"uni✓code", "uni%E2%9C%93code"},
		{"100%", "100%25"},
		{"a+b", "a%2Bb"},
		{"a?b#c", "a%3Fb%23c"},
		{"", ""},
		{"AZaz09-_.~:", "AZaz09-_.~:"},
	}
	for _, tc := range cases {
		if got := EncodeSegment(tc.in); got != tc.want {
			t.Errorf("EncodeSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func FuzzEncodeSegment(f *testing.F) {
	f.Add("graph/follow/x.near")
	f.Add("ed25519:abc")
	f.Add("a b%?#+&=")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		enc := EncodeSegment(s)
		for i := 0; i < len(enc); i++ {
			c := enc[i]
			ok := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '%' ||
				c == '-' || c == '_' || c == '.' || c == '~' || c == ':'
			if !ok {
				t.Fatalf("EncodeSegment(%q) produced %q containing %q", s, enc, c)
			}
		}
		if len(s) > 0 && enc == "" {
			t.Fatalf("EncodeSegment(%q) produced empty output", s)
		}
	})
}

func TestBuildPath(t *testing.T) {
	if got := BuildPath("v0", "latest", "social.near", "james.near", "graph/follow/sleet.near"); got !=
		"/v0/latest/social.near/james.near/graph%2Ffollow%2Fsleet.near" {
		t.Fatalf("BuildPath = %q", got)
	}
	if got := BuildPath(); got != "/" {
		t.Fatalf("BuildPath() = %q", got)
	}
	if got := BuildPath("v1", "public_key", "ed25519:5abc"); got != "/v1/public_key/ed25519:5abc" {
		t.Fatalf("BuildPath = %q", got)
	}
}

func TestQueryEncode(t *testing.T) {
	q := Query{{"limit", "10"}, {"desc", "true"}}
	q.Set("apiKey", "k")
	got := q.Encode()
	if got != "apiKey=k&desc=true&limit=10" {
		t.Fatalf("Query.Encode = %q", got)
	}
}
