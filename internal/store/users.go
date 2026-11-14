package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound      = errors.New("nie znaleziono")
	ErrEmailTaken    = errors.New("email jest już zajęty")
	ErrUsernameTaken = errors.New("nazwa użytkownika jest już zajęta")
)

// OwnerUsername to konto, w którego kontekście działa tryb AUTH_ENABLED=false.
const (
	OwnerUsername = "owner"
	ownerEmail    = "owner@localhost"
)

// User to wiersz tabeli users.
type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

const userColumns = `id, username, email, password_hash, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateUser zakłada konto. Zwraca ErrEmailTaken albo ErrUsernameTaken przy
// konflikcie (bez względu na wielkość liter).
func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3)
		 RETURNING `+userColumns, username, email, passwordHash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "users_email_key":
				return User{}, ErrEmailTaken
			case "users_username_key":
				return User{}, ErrUsernameTaken
			}
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// UserByEmail szuka konta po adresie bez względu na wielkość liter.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email))
}

// UserByID zwraca konto o danym identyfikatorze.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// EnsureOwner idempotentnie tworzy konto owner (bez hasła, więc nie da się
// na nie zalogować) i zwraca jego identyfikator.
func (s *Store) EnsureOwner(ctx context.Context) (int64, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (username, email, password_hash) VALUES ($1, $2, '')
		 ON CONFLICT DO NOTHING`, OwnerUsername, ownerEmail)
	if err != nil {
		return 0, fmt.Errorf("ensure owner: %w", err)
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE lower(username) = $1`, OwnerUsername).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensure owner: %w", err)
	}
	return id, nil
}
