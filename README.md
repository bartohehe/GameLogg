# GameLogg

Self-hosted tracker gier oraz filmów, seriali i anime. Gry stoją na półkach jak pudełka z grzbietami w kolorach okładek, a ocenę liczysz, ustawiając grę w rankingach sześciu kategorii zamiast wpisywać liczbę z głowy.

Jedna binarka Go (API + wbudowany frontend React) i PostgreSQL. Stawiasz to u siebie przez `docker compose up -d` na mini-PC albo domowym serwerze.

## Co potrafi

- **Półki gier:** w trakcie, planowane, ukończone, wstrzymane, lista życzeń, porzucone. Metadane i okładki z IGDB, kolory grzbietów liczone z okładki.
- **Ocena z rankingów:** każdą ukończoną grę wstawiasz między inne w sześciu kategoriach (gameplay, fabuła, grafika, audio, regrywalność, trudność). Ocena 0–100 liczy się sama ze średniej ważonej miejsc.
- **Odkryj:** gry w trakcie, podsumowanie roku, popularne premiery z IGDB i polecenia dobrane do najwyżej ocenionych gier.
- **Filmy, seriale i anime:** TMDB i MyAnimeList (Jikan), licznik odcinków, ręczna ocena.
- **Tłumaczenie opisów** gier z angielskiego (MyMemory) z dostępem do oryginału.
- **Tryb jednego użytkownika** bez logowania (`AUTH_ENABLED=false`).
- Interfejs po polsku, działa na telefonie, obsługa z klawiatury i czytników ekranu.

## Szybki start

Potrzebujesz Dockera z Compose v2.

```bash
git clone https://github.com/bartohehe/GameLogg.git
```

```bash
cd GameLogg && cp .env.example .env
```

Uzupełnij `.env`:

1. Ustaw to samo hasło bazy w `POSTGRES_PASSWORD` i w `DATABASE_URL` (w miejscu `change-me`). Najprościej użyć hasła bez znaków specjalnych, np. z `openssl rand -hex 16`, bo w `DATABASE_URL` musiałyby być zakodowane.
2. Wpisz `JWT_SECRET` (co najmniej 32 znaki):

   ```bash
   openssl rand -hex 32
   ```

3. Dopisz klucze IGDB (opis niżej). Bez nich aplikacja wystartuje, ale nie wyszuka gier.

Uruchom:

```bash
docker compose up -d
```

Otwórz `http://localhost:8080` i załóż konto. Grasz sam? Ustaw `AUTH_ENABLED=false`, a GameLogg pominie logowanie i będzie działał na jednym koncie.

### Gotowy obraz zamiast budowania

GitHub Actions publikuje obraz dla `linux/amd64` i `linux/arm64` (np. Raspberry Pi) w GitHub Container Registry: `ghcr.io/bartohehe/gamelogg`. Tagi: `latest` i `main` (ostatni stan gałęzi main), `1.2.3` i `1.2` dla wydań oraz `sha-…` dla każdego commita.

Żeby nie budować obrazu u siebie, w `docker-compose.yml` zamień w serwisie `app` sekcję `build:` i linię `image:` na:

```yaml
    image: ghcr.io/bartohehe/gamelogg:latest
```

Komentarze w `.env` trzymaj w osobnych liniach. Docker Compose przy pustej wartości wczytuje komentarz z tej samej linii jako wartość zmiennej; GameLogg wykrywa to przy starcie i zgłasza błąd.

## Klucze IGDB

IGDB należy do Twitcha, więc klucze wydaje Twitch Developer Console.

1. Zaloguj się na [dev.twitch.tv/console/apps](https://dev.twitch.tv/console/apps). Konto Twitch musi mieć włączone uwierzytelnianie dwuskładnikowe.
2. Kliknij **Register Your Application**:
   - nazwa: dowolna, np. `GameLogg mój serwer`,
   - OAuth Redirect URLs: `http://localhost`,
   - kategoria: **Application Integration**,
   - Client Type: **Confidential**.
3. Wejdź w **Manage** przy aplikacji. Skopiuj **Client ID**, potem kliknij **New Secret** i skopiuj sekret.
4. Wpisz oba do `.env`:

   ```
   IGDB_CLIENT_ID=twój-client-id
   IGDB_CLIENT_SECRET=twój-sekret
   ```

5. Przeładuj kontener aplikacji:

   ```bash
   docker compose up -d app
   ```

`docker compose restart app` nie wystarczy: restart nie wczytuje ponownie `.env`. `up -d` odtwarza kontener, gdy zmieniła się konfiguracja.

## Klucz TMDB

Filmy i seriale pochodzą z TMDB. Klucz jest opcjonalny: bez niego działa samo anime z MyAnimeList.

1. Załóż konto na [themoviedb.org](https://www.themoviedb.org/signup).
2. W ustawieniach konta wejdź w **API** i poproś o klucz (typ **Developer**, użytek prywatny).
3. Skopiuj **API Key** albo **API Read Access Token**. Działa każdy z nich.
4. Wpisz go jako `TMDB_API_KEY` w `.env` i przeładuj kontener: `docker compose up -d app`.

## Konfiguracja

Wszystko ustawia się zmiennymi w `.env`. Błędna konfiguracja zatrzymuje start z listą wszystkich problemów naraz.

| Zmienna | Domyślnie | Opis |
| --- | --- | --- |
| `DATABASE_URL` | – | Adres PostgreSQL. W Compose host to `postgres`. |
| `POSTGRES_PASSWORD` | – | Hasło bazy dla kontenera `postgres`. |
| `JWT_SECRET` | – | Klucz podpisu sesji, min. 32 znaki. Wymagany przy `AUTH_ENABLED=true`. |
| `IGDB_CLIENT_ID`, `IGDB_CLIENT_SECRET` | puste | Klucze IGDB. Podaje się oba albo żadnego. |
| `TMDB_API_KEY` | puste | Klucz TMDB (v3) albo token v4. |
| `AUTH_ENABLED` | `true` | `false` = jeden użytkownik bez logowania. |
| `MEDIA_ENABLED` | `true` | Filmy, seriale i anime. |
| `TRANSLATION_ENABLED` | `true` | Tłumaczenie opisów gier przez MyMemory. |
| `PORT` | `8080` | Port HTTP aplikacji. |
| `APP_ENV` | `production` | `dev` = czytelne logi tekstowe z poziomem debug. |

Stan flag widać w aplikacji na ekranie **Profil → Ten serwer**.

Darmowe tłumaczenia MyMemory mają dzienny limit (ok. 5000 znaków). Po jego wyczerpaniu karty gier pokazują opis po angielsku, a przetłumaczone wcześniej opisy zostają w bazie na 30 dni.

## Reverse proxy i HTTPS (Caddy)

GameLogg nie obsługuje TLS sam. Postaw przed nim reverse proxy. Przykład z [Caddy](https://caddyserver.com/), który sam załatwi certyfikat Let's Encrypt:

```caddyfile
gamelogg.example.com {
	reverse_proxy localhost:8080
}
```

Caddy przekazuje nagłówek `X-Forwarded-Proto: https`, więc GameLogg ustawia ciasteczko sesji z flagą `Secure`.

Jeśli GameLogg ma być dostępny tylko przez proxy, w `docker-compose.yml` zmień mapowanie portu na `"127.0.0.1:8080:8080"`.

Rejestracja jest otwarta dla każdego, kto dotrze do serwera. Jeśli wystawiasz GameLogg do internetu, a korzystasz z niego sam, rozważ `AUTH_ENABLED=false` i dostęp chroniony przez proxy (np. `basic_auth` w Caddy) albo VPN.

## Kopia zapasowa

Wszystkie dane są w PostgreSQL. Kopia w formacie `pg_dump`:

```bash
docker compose exec -T postgres pg_dump -U gamelogg -d gamelogg -Fc > gamelogg-$(date +%F).dump
```

Odtworzenie (nadpisuje obecne dane):

```bash
docker compose stop app
```

```bash
docker compose exec -T postgres pg_restore -U gamelogg -d gamelogg --clean --if-exists < gamelogg-2026-11-14.dump
```

```bash
docker compose start app
```

Codzienna kopia z crona (o 3:30, katalog `/srv/backup`):

```cron
30 3 * * * cd /srv/GameLogg && docker compose exec -T postgres pg_dump -U gamelogg -d gamelogg -Fc > /srv/backup/gamelogg-$(date +\%F).dump
```

## Aktualizacja

```bash
git pull && docker compose up -d --build
```

Z gotowym obrazem z GHCR:

```bash
docker compose pull && docker compose up -d
```

Migracje bazy uruchamiają się same przy starcie aplikacji. Wersję widoczną w profilu ustawisz przy budowie:

```bash
GAMELOGG_VERSION=$(git describe --tags --always) docker compose up -d --build
```

## Rozwój

Potrzebujesz Go 1.26+, Node 24, Dockera i GNU make.

```bash
make dev
```

Uruchamia PostgreSQL w Dockerze (`docker-compose.dev.yml`), API na `:8080` i frontend z HMR na `http://localhost:5173`.

| Polecenie | Co robi |
| --- | --- |
| `make test` | `go vet`, wszystkie testy Go (z integracyjnymi na prawdziwym Postgresie) i testy frontendu |
| `make test-short` | to samo bez Dockera: pomija testy integracyjne |
| `make lint` | `go vet` i ESLint |
| `make build` | frontend + binarka z wbudowanym frontendem w `bin/gamelogg` |
| `make docker` | obraz produkcyjny |

Struktura:

```
cmd/gamelogg/         punkt wejścia, healthcheck kontenera
internal/config/      zmienne środowiskowe i ich walidacja
internal/store/       całe SQL (pgx, bez ORM)
internal/migrations/  migracje goose, wbudowane w binarkę
internal/api/         handlery HTTP, middleware, DTO
internal/auth/        bcrypt i tokeny sesji
internal/igdb/        klient IGDB z tokenem Twitch
internal/tmdb/        klient TMDB
internal/jikan/       klient Jikan (MyAnimeList) z limitem żądań
internal/translate/   tłumaczenie MyMemory z cache w Postgresie
internal/ranking/     ocena z miejsc w rankingach (czyste funkcje)
internal/spine/       kolory grzbietów z okładek
web/                  frontend: React 19, TypeScript, Vite
```

Logika oceny jest w Go (`internal/ranking`) i w TypeScripcie (`web/src/lib/ranking.ts`, podgląd na żywo w oknie oceny). Oba testy czytają te same przypadki z `internal/ranking/testdata/cases.json`, więc obie wersje muszą liczyć tak samo.
