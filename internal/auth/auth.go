// Package auth obsługuje hasła (bcrypt) i tokeny sesji (JWT HS256).
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLen to minimalna długość hasła w znakach.
	MinPasswordLen = 8
	// MaxPasswordBytes to limit bcrypt: dłuższe hasła byłyby po cichu ucinane.
	MaxPasswordBytes = 72

	bcryptCost = 12
	issuer     = "gamelogg"
)

var (
	ErrPasswordTooShort = fmt.Errorf("hasło musi mieć co najmniej %d znaków", MinPasswordLen)
	ErrPasswordTooLong  = fmt.Errorf("hasło może mieć najwyżej %d bajtów", MaxPasswordBytes)
	ErrInvalidToken     = errors.New("nieprawidłowy albo wygasły token")
)

// ValidatePassword sprawdza długość nowego hasła.
func ValidatePassword(pw string) error {
	if len([]rune(pw)) < MinPasswordLen {
		return ErrPasswordTooShort
	}
	if len(pw) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

// HashPassword zwraca hash bcrypt hasła.
func HashPassword(pw string) (string, error) {
	if err := ValidatePassword(pw); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(h), nil
}

// dummyHash wyrównuje czas logowania, gdy użytkownik nie istnieje,
// żeby po czasie odpowiedzi nie dało się zgadywać istniejących adresów.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("gamelogg-dummy-password"), bcryptCost)

// CheckPassword porównuje hasło z hashem. Pusty hash (np. konto owner
// w trybie bez logowania) nigdy nie pasuje.
func CheckPassword(hash, pw string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(pw))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// SpendCheckTime zużywa tyle czasu, co sprawdzenie hasła. Wołane, gdy
// użytkownika nie znaleziono.
func SpendCheckTime(pw string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(pw))
}

// Tokens wystawia i weryfikuje tokeny sesji.
type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokens tworzy wystawcę tokenów z sekretem HMAC i czasem życia sesji.
func NewTokens(secret string, ttl time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// TTL zwraca czas życia tokenu.
func (t *Tokens) TTL() time.Duration { return t.ttl }

// Issue wystawia token dla użytkownika i zwraca go razem z datą wygaśnięcia.
func (t *Tokens) Issue(userID int64) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(t.ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("podpis tokenu: %w", err)
	}
	return s, exp, nil
}

// Session to dane z poprawnego tokenu sesji.
type Session struct {
	UserID   int64
	IssuedAt time.Time
}

// Parse weryfikuje token i zwraca identyfikator użytkownika.
func (t *Tokens) Parse(token string) (int64, error) {
	s, err := t.ParseSession(token)
	return s.UserID, err
}

// ParseSession weryfikuje token i zwraca użytkownika oraz moment wystawienia
// (potrzebny, żeby zmiana hasła unieważniała starsze sesje).
func (t *Tokens) ParseSession(token string) (Session, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil || claims.IssuedAt == nil {
		return Session{}, ErrInvalidToken
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return Session{}, ErrInvalidToken
	}
	return Session{UserID: id, IssuedAt: claims.IssuedAt.Time}, nil
}
