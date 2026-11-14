package store

import (
	"context"
	"errors"
	"testing"
)

func migratedStore(t *testing.T) *Store {
	t.Helper()
	s := openTestStore(t)
	if err := s.Migrate(context.Background(), discardLogger()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := s.pool.Exec(context.Background(), `TRUNCATE users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return s
}

func TestCreateAndFindUser(t *testing.T) {
	s := migratedStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "Bartek", "Bartek@Example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 || u.Username != "Bartek" || u.CreatedAt.IsZero() {
		t.Errorf("u = %+v", u)
	}

	byEmail, err := s.UserByEmail(ctx, "bartek@example.COM")
	if err != nil || byEmail.ID != u.ID {
		t.Errorf("UserByEmail = %+v, %v", byEmail, err)
	}
	byID, err := s.UserByID(ctx, u.ID)
	if err != nil || byID.Email != "Bartek@Example.com" {
		t.Errorf("UserByID = %+v, %v", byID, err)
	}

	if _, err := s.UserByEmail(ctx, "nikt@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("brak użytkownika: err = %v", err)
	}
	if _, err := s.UserByID(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("brak użytkownika po id: err = %v", err)
	}
}

func TestCreateUserConflicts(t *testing.T) {
	s := migratedStore(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "jan", "jan@example.com", "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "inny", "JAN@example.com", "h"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplikat emaila: err = %v", err)
	}
	if _, err := s.CreateUser(ctx, "JAN", "jan2@example.com", "h"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplikat nazwy: err = %v", err)
	}
}

func TestEnsureOwnerIdempotent(t *testing.T) {
	s := migratedStore(t)
	ctx := context.Background()
	id1, err := s.EnsureOwner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.EnsureOwner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("EnsureOwner zwrócił %d i %d", id1, id2)
	}
	u, err := s.UserByID(ctx, id1)
	if err != nil || u.Username != OwnerUsername || u.PasswordHash != "" {
		t.Errorf("owner = %+v, %v", u, err)
	}
}
