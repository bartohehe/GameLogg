package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want error
	}{
		{"za krótkie", "1234567", ErrPasswordTooShort},
		{"8 znaków", "12345678", nil},
		{"8 znaków z polskimi literami", "zażółćęś", nil},
		{"72 bajty", strings.Repeat("a", 72), nil},
		{"73 bajty", strings.Repeat("a", 73), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePassword(tt.pw); !errors.Is(got, tt.want) {
				t.Errorf("ValidatePassword = %v, oczekiwano %v", got, tt.want)
			}
		})
	}
}

func TestHashAndCheck(t *testing.T) {
	h, err := HashPassword("poprawne-haslo")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "poprawne-haslo") {
		t.Error("poprawne hasło odrzucone")
	}
	if CheckPassword(h, "zle-haslo") {
		t.Error("złe hasło przyjęte")
	}
	if CheckPassword("", "cokolwiek") {
		t.Error("pusty hash nie może pasować")
	}
}

func TestTokens(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	tk := NewTokens("0123456789abcdef0123456789abcdef", time.Hour)
	tk.now = func() time.Time { return now }

	tok, exp, err := tk.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(now.Add(time.Hour)) {
		t.Errorf("exp = %v", exp)
	}

	id, err := tk.Parse(tok)
	if err != nil || id != 42 {
		t.Fatalf("Parse = %d, %v", id, err)
	}

	t.Run("wygasły", func(t *testing.T) {
		later := *tk
		later.now = func() time.Time { return now.Add(2 * time.Hour) }
		if _, err := later.Parse(tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("inny sekret", func(t *testing.T) {
		other := NewTokens("ffffffffffffffffffffffffffffffff", time.Hour)
		other.now = tk.now
		if _, err := other.Parse(tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("alg none", func(t *testing.T) {
		claims := jwt.RegisteredClaims{Issuer: issuer, Subject: "1", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
		s, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tk.Parse(s); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("token bez podpisu przyjęty: %v", err)
		}
	})

	t.Run("śmieci", func(t *testing.T) {
		if _, err := tk.Parse("nie.jest.tokenem"); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("err = %v", err)
		}
	})
}
