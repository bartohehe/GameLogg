-- +goose Up
CREATE TYPE game_status AS ENUM ('planned', 'in_progress', 'completed', 'on_hold', 'wishlist', 'abandoned');
CREATE TYPE ranking_category AS ENUM ('gameplay', 'story', 'graphics', 'audio', 'replayability', 'difficulty');
CREATE TYPE media_source AS ENUM ('tmdb', 'jikan');
CREATE TYPE media_type AS ENUM ('film', 'serial', 'anime');
CREATE TYPE media_status AS ENUM ('backlog', 'watching', 'watched');

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      text        NOT NULL CHECK (length(username) BETWEEN 1 AND 64),
    email         text        NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
-- Unikalność bez względu na wielkość liter: Jan@x.pl i jan@x.pl to ten sam adres.
CREATE UNIQUE INDEX users_username_key ON users (lower(username));
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

-- Cache metadanych z IGDB, współdzielony przez wszystkich użytkowników.
CREATE TABLE games (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    igdb_id      bigint      NOT NULL UNIQUE,
    title        text        NOT NULL,
    cover_url    text,
    release_date date,
    developer    text,
    genres       text[]      NOT NULL DEFAULT '{}',
    platforms    text[]      NOT NULL DEFAULT '{}',
    summary      text,
    igdb_rating  smallint CHECK (igdb_rating BETWEEN 0 AND 100),
    spine_color  text        NOT NULL CHECK (spine_color ~ '^#[0-9A-Fa-f]{6}$'),
    spine_accent text        NOT NULL CHECK (spine_accent ~ '^#[0-9A-Fa-f]{6}$'),
    spine_ink    text        NOT NULL CHECK (spine_ink ~ '^#[0-9A-Fa-f]{6}$'),
    fetched_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_games (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    game_id         bigint      NOT NULL REFERENCES games (id) ON DELETE RESTRICT,
    status          game_status NOT NULL,
    platform        text        NOT NULL DEFAULT '',
    -- Wyliczana z rankingów kategorii, nigdy wpisywana ręcznie.
    score           smallint CHECK (score BETWEEN 0 AND 100),
    note            text        NOT NULL DEFAULT '',
    completed_count integer     NOT NULL DEFAULT 0 CHECK (completed_count >= 0),
    added_at        timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, game_id),
    -- Cel klucza obcego z category_rankings: wpis rankingu musi należeć do tego samego użytkownika.
    UNIQUE (id, user_id),
    -- W rankingach są tylko ukończone gry, więc tylko one mogą mieć ocenę.
    CONSTRAINT user_games_score_only_completed CHECK (score IS NULL OR status = 'completed')
);
CREATE INDEX user_games_user_status_idx ON user_games (user_id, status);

CREATE TABLE category_rankings (
    user_id      bigint           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category     ranking_category NOT NULL,
    user_game_id bigint           NOT NULL,
    -- 0 = najlepsza.
    position     integer          NOT NULL CHECK (position >= 0),
    PRIMARY KEY (user_id, category, user_game_id),
    FOREIGN KEY (user_game_id, user_id) REFERENCES user_games (id, user_id) ON DELETE CASCADE,
    -- Odroczone do commitu, żeby przestawianie kolejności w jednej transakcji
    -- nie potykało się o chwilowe duplikaty pozycji.
    CONSTRAINT category_rankings_position_key UNIQUE (user_id, category, position) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX category_rankings_user_game_idx ON category_rankings (user_game_id);

CREATE TABLE user_media (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          bigint       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source           media_source NOT NULL,
    external_id      text         NOT NULL,
    type             media_type   NOT NULL,
    title            text         NOT NULL,
    year             integer,
    creator          text         NOT NULL DEFAULT '',
    genres           text[]       NOT NULL DEFAULT '{}',
    cover_url        text,
    runtime_min      integer CHECK (runtime_min > 0),
    episodes         integer CHECK (episodes > 0),
    watched_episodes integer CHECK (watched_episodes >= 0),
    status           media_status NOT NULL,
    -- Wpisywana ręcznie.
    score            smallint CHECK (score BETWEEN 0 AND 100),
    note             text         NOT NULL DEFAULT '',
    spine_color      text         NOT NULL CHECK (spine_color ~ '^#[0-9A-Fa-f]{6}$'),
    spine_accent     text         NOT NULL CHECK (spine_accent ~ '^#[0-9A-Fa-f]{6}$'),
    spine_ink        text         NOT NULL CHECK (spine_ink ~ '^#[0-9A-Fa-f]{6}$'),
    added_at         timestamptz  NOT NULL DEFAULT now(),
    updated_at       timestamptz  NOT NULL DEFAULT now(),
    UNIQUE (user_id, source, external_id),
    CONSTRAINT user_media_watched_le_episodes
        CHECK (watched_episodes IS NULL OR episodes IS NULL OR watched_episodes <= episodes)
);
CREATE INDEX user_media_user_status_idx ON user_media (user_id, status);

-- Wpisy starsze niż 30 dni aplikacja traktuje jak brak.
CREATE TABLE translation_cache (
    source_hash text        NOT NULL,
    lang        text        NOT NULL,
    translated  text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source_hash, lang)
);

-- +goose Down
DROP TABLE translation_cache;
DROP TABLE user_media;
DROP TABLE category_rankings;
DROP TABLE user_games;
DROP TABLE games;
DROP TABLE users;
DROP TYPE media_status;
DROP TYPE media_type;
DROP TYPE media_source;
DROP TYPE ranking_category;
DROP TYPE game_status;
