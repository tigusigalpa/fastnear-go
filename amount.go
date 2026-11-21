package fastnear

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/tigusigalpa/fastnear-go/fastnearerr"
)

// yoctoPerNEAR is 10^24: the number of yoctoNEAR in one NEAR.
var yoctoPerNEAR = new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)

// Amount is a NEAR balance stored as integer yoctoNEAR. It never uses
// floating point: API balances are u128 strings and must not lose
// precision.
type Amount struct {
	yocto *big.Int
}

// FromYocto parses a base-10 yoctoNEAR string (e.g. "1500000000000000000000000").
func FromYocto(s string) (*Amount, error) {
	if s == "" || !isDigits(s) {
		return nil, &fastnearerr.Error{Code: "invalid_amount", Message: fmt.Sprintf("invalid yoctoNEAR amount %q", s)}
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return nil, &fastnearerr.Error{Code: "invalid_amount", Message: fmt.Sprintf("invalid yoctoNEAR amount %q", s)}
	}
	return &Amount{yocto: v}, nil
}

// FromNEAR parses a decimal NEAR string ("1.5") exactly, without
// floating point. Up to 24 fractional digits are accepted.
func FromNEAR(s string) (*Amount, error) {
	fail := func() (*Amount, error) {
		return nil, &fastnearerr.Error{Code: "invalid_amount", Message: fmt.Sprintf("invalid NEAR amount %q", s)}
	}
	if s == "" {
		return fail()
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return fail()
	}
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if intPart == "" && fracPart == "" {
		return fail()
	}
	if intPart != "" && !isDigits(intPart) {
		return fail()
	}
	if fracPart != "" && !isDigits(fracPart) {
		return fail()
	}
	if len(fracPart) > 24 {
		return fail()
	}
	fracPart += strings.Repeat("0", 24-len(fracPart))
	v, ok := new(big.Int).SetString(intPart+fracPart, 10)
	if !ok {
		return fail()
	}
	return &Amount{yocto: v}, nil
}

// MustFromNEAR is FromNEAR that panics on error. Use only for
// constants in tests and examples.
func MustFromNEAR(s string) *Amount {
	a, err := FromNEAR(s)
	if err != nil {
		panic(err)
	}
	return a
}

// Yocto returns the exact yoctoNEAR string.
func (a *Amount) Yocto() string {
	if a == nil || a.yocto == nil {
		return "0"
	}
	return a.yocto.String()
}

// NEAR returns the exact decimal NEAR string without exponent
// notation, e.g. "1.5".
func (a *Amount) NEAR() string {
	if a == nil || a.yocto == nil {
		return "0"
	}
	v := new(big.Int).Abs(a.yocto)
	sign := ""
	if a.yocto.Sign() < 0 {
		sign = "-"
	}
	q, r := new(big.Int).QuoRem(v, yoctoPerNEAR, new(big.Int))
	if r.Sign() == 0 {
		return sign + q.String()
	}
	frac := r.String()
	frac = strings.Repeat("0", 24-len(frac)) + frac
	frac = strings.TrimRight(frac, "0")
	return sign + q.String() + "." + frac
}

// String returns e.g. "1.5 NEAR".
func (a *Amount) String() string { return a.NEAR() + " NEAR" }

// Add returns a + b.
func (a *Amount) Add(b *Amount) *Amount {
	return &Amount{yocto: new(big.Int).Add(a.integer(), b.integer())}
}

// Sub returns a - b (may go negative).
func (a *Amount) Sub(b *Amount) *Amount {
	return &Amount{yocto: new(big.Int).Sub(a.integer(), b.integer())}
}

// Cmp compares a and b: -1, 0 or +1.
func (a *Amount) Cmp(b *Amount) int { return a.integer().Cmp(b.integer()) }

func (a *Amount) integer() *big.Int {
	if a == nil || a.yocto == nil {
		return new(big.Int)
	}
	return a.yocto
}

// IsZero reports whether the amount is zero.
func (a *Amount) IsZero() bool {
	return a == nil || a.yocto == nil || a.yocto.Sign() == 0
}

// TokenAmount is a fungible-token balance: a raw integer string plus
// the token's decimals from ft_metadata.
type TokenAmount struct {
	Raw      string
	Decimals uint8
}

// Format renders the human value truncated to precision fractional
// digits (never rounded up).
// A negative precision is treated as zero; invalid Raw values format as "0".
func (t TokenAmount) Format(precision int) string {
	v, ok := new(big.Int).SetString(t.Raw, 10)
	if !ok {
		return "0"
	}
	dec := int(t.Decimals)
	if precision < 0 {
		precision = 0
	}
	if precision > dec {
		precision = dec
	}
	sign := ""
	if v.Sign() < 0 {
		sign = "-"
		v.Abs(v)
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(dec)), nil)
	q, r := new(big.Int).QuoRem(v, divisor, new(big.Int))
	if precision == 0 {
		if q.Sign() == 0 {
			return "0"
		}
		return sign + q.String()
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(dec-precision)), nil)
	r.Quo(r, scale)
	frac := r.String()
	frac = strings.Repeat("0", precision-len(frac)) + frac
	return sign + q.String() + "." + frac
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
