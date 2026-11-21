package fastnear

import "testing"

func TestNegativeAmountsAndZeroValueArithmetic(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"1", "1.5", "-0.5"}, {"1", "3.25", "-2.25"}, {"0", "1", "-1"},
	} {
		got := MustFromNEAR(tc.a).Sub(MustFromNEAR(tc.b))
		if got.NEAR() != tc.want {
			t.Fatalf("%s - %s = %s", tc.a, tc.b, got.NEAR())
		}
	}
	var zero Amount
	var nilAmount *Amount
	if zero.Add(nilAmount).Yocto() != "0" || nilAmount.Cmp(&zero) != 0 {
		t.Fatal("zero-value arithmetic failed")
	}
	if zero.Sub(MustFromNEAR("0.5")).NEAR() != "-0.5" {
		t.Fatal("zero subtraction failed")
	}
	for _, tc := range []struct {
		raw       string
		decimals  uint8
		precision int
		want      string
	}{
		{"123", 2, -1, "1"}, {"-123", 2, 2, "-1.23"}, {"-5", 2, 2, "-0.05"}, {"0", 0, -100, "0"},
	} {
		if got := (TokenAmount{Raw: tc.raw, Decimals: tc.decimals}).Format(tc.precision); got != tc.want {
			t.Fatalf("format(%+v) = %q", tc, got)
		}
	}
}

func TestFromNEAR(t *testing.T) {
	cases := []struct{ in, yocto, near string }{
		{"1", "1000000000000000000000000", "1"},
		{"1.5", "1500000000000000000000000", "1.5"},
		{"0.000000000000000000000001", "1", "0.000000000000000000000001"},
		{"123456789.123456789", "123456789123456789000000000000000", "123456789.123456789"},
		{"0", "0", "0"},
	}
	for _, tc := range cases {
		a, err := FromNEAR(tc.in)
		if err != nil {
			t.Fatalf("FromNEAR(%q): %v", tc.in, err)
		}
		if a.Yocto() != tc.yocto {
			t.Errorf("FromNEAR(%q).Yocto() = %q, want %q", tc.in, a.Yocto(), tc.yocto)
		}
		if a.NEAR() != tc.near {
			t.Errorf("FromNEAR(%q).NEAR() = %q, want %q", tc.in, a.NEAR(), tc.near)
		}
	}
}

func TestFromNEARInvalid(t *testing.T) {
	for _, in := range []string{"", "abc", "1.2.3", "1e5", "NaN", "Inf", "-1", "+1", "0.0000000000000000000000001", ".", "1..", " "} {
		if _, err := FromNEAR(in); err == nil {
			t.Errorf("FromNEAR(%q) must fail", in)
		}
	}
}

func FuzzFromNEAR(f *testing.F) {
	for _, s := range []string{"1", "1.5", "0.000000000000000000000001", "abc", "1.2.3", "9999999999999999999999.5"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		a, err := FromNEAR(s)
		if err != nil {
			return
		}
		if a == nil || a.yocto == nil {
			t.Fatalf("FromNEAR(%q) returned nil internals", s)
		}
		if a.yocto.Sign() < 0 {
			t.Fatalf("FromNEAR(%q) produced negative amount", s)
		}
	})
}

func TestFromYocto(t *testing.T) {
	a, err := FromYocto("1500000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if a.NEAR() != "1.5" {
		t.Fatalf("NEAR() = %q", a.NEAR())
	}
	for _, bad := range []string{"", "-5", "1.5", "abc"} {
		if _, err := FromYocto(bad); err == nil {
			t.Errorf("FromYocto(%q) must fail", bad)
		}
	}
}

func TestAmountArithmetic(t *testing.T) {
	a := MustFromNEAR("2")
	b := MustFromNEAR("0.5")
	if a.Add(b).NEAR() != "2.5" {
		t.Fatalf("Add = %q", a.Add(b).NEAR())
	}
	if a.Sub(b).NEAR() != "1.5" {
		t.Fatalf("Sub = %q", a.Sub(b).NEAR())
	}
	if a.Cmp(b) != 1 || b.Cmp(a) != -1 {
		t.Fatal("Cmp wrong")
	}
	if !MustFromNEAR("0").IsZero() || a.IsZero() {
		t.Fatal("IsZero wrong")
	}
	if a.String() != "2 NEAR" {
		t.Fatalf("String = %q", a.String())
	}
}

func TestTokenAmountFormat(t *testing.T) {
	tok := TokenAmount{Raw: "1234567", Decimals: 6}
	if got := tok.Format(2); got != "1.23" {
		t.Fatalf("Format = %q", got)
	}
	if got := tok.Format(6); got != "1.234567" {
		t.Fatalf("Format = %q", got)
	}
	// Truncate, not round up.
	tok2 := TokenAmount{Raw: "1999999", Decimals: 6}
	if got := tok2.Format(2); got != "1.99" {
		t.Fatalf("Format = %q (must truncate)", got)
	}
	if got := tok2.Format(0); got != "1" {
		t.Fatalf("Format = %q", got)
	}
}
