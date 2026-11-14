-- +goose Up
-- Moment ostatniej zmiany hasła: tokeny sesji wystawione wcześniej przestają działać,
-- więc zmiana hasła wylogowuje pozostałe urządzenia.
ALTER TABLE users ADD COLUMN password_changed_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN password_changed_at;
