# GameLogg v2: Go + React + PostgreSQL

Budujesz od zera nową wersję GameLogga: self-hosted tracker gier oraz filmów, seriali i anime, z redesignem opartym na metaforze półki z grzbietami pudełek. Projekt jest open source, a typowy użytkownik stawia go u siebie przez `docker compose up -d` na mini-PC lub domowym serwerze.

Stara wersja (React + ASP.NET + Redis) jest w `https://github.com/bartohehe/gamelog-app`. Możesz tam zajrzeć po szczegóły zapytań do IGDB, TMDB, Jikan i MyMemory (`backend/Services/*.cs`), ale nie przenoś jej architektury ani kodu frontendu. **Multiplayera nie budujemy w ogóle**: żadnych tabel, endpointów, kluczy Riot ani Steam, żadnej pozycji w nawigacji.

## Jak pracujesz

- Pracuj etapami (sekcja „Etapy” na końcu). Po każdym etapie: `go vet ./...`, `go test ./...`, `npm run build` i `npm run lint` we frontendzie, potem commit z opisem etapu. Nie zaczynaj kolejnego etapu, jeśli poprzedni nie przechodzi.
- Zanim zaczniesz etap 0, przeczytaj cały ten plik i mockupy w `design/`. Jeśli coś w specyfikacji jest sprzeczne albo niejasne, zapytaj, zanim napiszesz kod.
- Interfejs jest w całości po polsku. Daty formatuj przez `Intl` z `pl-PL`.
- Nie dodawaj zależności spoza listy w „Stacku” bez pytania.

## Stack

**Backend (Go 1.23+)**
- Router: `github.com/go-chi/chi/v5`
- Baza: `github.com/jackc/pgx/v5` (pgxpool), zapytania SQL pisane ręcznie w pakiecie `internal/store`. Bez ORM-a.
- Migracje: `github.com/pressly/goose/v3`, pliki SQL wbudowane przez `embed` i odpalane przy starcie aplikacji.
- Auth: `golang.org/x/crypto/bcrypt` i `github.com/golang-jwt/jwt/v5`.
- Logi: `log/slog` (JSON w produkcji, tekst w trybie dev).
- Konfiguracja: wyłącznie zmienne środowiskowe, wczytywane w jednym pakiecie `internal/config` i walidowane przy starcie.

**Frontend**
- React 19, TypeScript (strict) i Vite.
- `react-router` do routingu, `@tanstack/react-query` do danych z API.
- Stylowanie: zwykły CSS z custom properties (plik `tokens.css`) i CSS Modules dla komponentów. Bez Tailwinda i bez bibliotek komponentów.
- Font: Archivo, hostowany lokalnie przez `@fontsource-variable/archivo`. Potrzebna jest oś szerokości (`wdth` 62–125), więc importuj wariant z osią wdth i sprawdź, że `font-stretch` naprawdę działa. Aplikacja ma działać bez internetu poza zewnętrznymi API, dlatego żadnych Google Fonts z CDN.

**Wdrożenie**
- Jedna binarka Go serwuje API pod `/api/*` i zbudowany frontend (`go:embed` katalogu `web/dist`), z fallbackiem SPA na `index.html`.
- Wieloetapowy `Dockerfile`: najpierw build frontendu na node, potem build Go, na końcu obraz `distroless` lub `alpine`.
- `docker-compose.yml` ma dwa serwisy: `app` i `postgres:16`, z wolumenem na dane i healthcheckiem bazy. Bez Redisa i bez nginx. Reverse proxy i TLS zostawiamy użytkownikowi (opisz w README przykład z Caddy).

## Struktura repo

```
cmd/gamelogg/main.go
internal/config/       env → struct, walidacja
internal/store/        całe SQL, transakcje
internal/migrations/   *.sql (goose, embed)
internal/api/          handlery HTTP, middleware, DTO
internal/auth/
internal/igdb/         klient IGDB + token Twitch
internal/tmdb/
internal/jikan/
internal/translate/    MyMemory + cache w Postgresie
internal/ranking/      liczenie ocen z rankingów (czyste funkcje, testy)
internal/spine/        kolory grzbietu z okładki
web/                   frontend (Vite)
design/                mockupy z redesignu, tylko do wglądu
```

## Zmienne środowiskowe (`.env.example`)

```
DATABASE_URL=postgres://gamelogg:change-me@postgres:5432/gamelogg?sslmode=disable
POSTGRES_PASSWORD=change-me
JWT_SECRET=                  # min. 32 znaki, start bez niego = błąd, gdy AUTH_ENABLED=true
IGDB_CLIENT_ID=
IGDB_CLIENT_SECRET=
TMDB_API_KEY=                # opcjonalny: bez niego wyszukiwanie filmów/seriali jest wyłączone, anime (Jikan) działa
AUTH_ENABLED=true
MEDIA_ENABLED=true
TRANSLATION_ENABLED=true
PORT=8080
```

Inne flagi ze starej wersji (MULTIPLAYER, SOCIAL, STATISTICS, REVIEWS, BACKLOG, ADVANCED_RATING) znikają. Ranking, notatki i wszystkie statusy są teraz częścią rdzenia.

`AUTH_ENABLED=false` oznacza tryb jednego użytkownika. Przy starcie aplikacja tworzy (idempotentnie) użytkownika `owner`, każde żądanie działa w jego kontekście, a ekrany logowania i rejestracji są niedostępne (przekierowanie na `/`).

## Model danych (PostgreSQL)

Zaprojektuj migracje na tej podstawie. Nazwy możesz dopracować, ale relacje i ograniczenia mają zostać.

- `users`: id, username (unique), email (unique), password_hash, created_at.
- `games`: cache metadanych z IGDB, współdzielony przez użytkowników. Pola: igdb_id (unique), title, cover_url, release_date, developer, genres text[], platforms text[], summary, igdb_rating, `spine_color`, `spine_accent`, `spine_ink` (hex), fetched_at.
- `user_games`: id, user_id, game_id, status (enum: `planned`, `in_progress`, `completed`, `on_hold`, `wishlist`, `abandoned`), platform, score int null (0–100, wyliczany, nie wpisywany ręcznie), note text, completed_count int default 0, added_at, updated_at. Unique (user_id, game_id).
- `category_rankings`: user_id, category (enum: `gameplay`, `story`, `graphics`, `audio`, `replayability`, `difficulty`), user_game_id, position int (0 = najlepsza). Unique (user_id, category, position) jako deferrable, unique (user_id, category, user_game_id).
- `user_media`: id, user_id, source (`tmdb` | `jikan`), external_id, type (`film`, `serial`, `anime`), title, year, creator, genres text[], cover_url, runtime_min null, episodes null, watched_episodes null, status (`backlog`, `watching`, `watched`), score null (0–100, wpisywany ręcznie), note, spine_color, spine_accent, spine_ink, added_at, updated_at. Unique (user_id, source, external_id).
- `translation_cache`: hash tekstu źródłowego, lang, translated, created_at. Wpisy starsze niż 30 dni traktuj jak brak.

## Logika, która musi być dokładna

### Ocena z rankingów (`internal/ranking`, testy jednostkowe obowiązkowe)

- Do rankingów trafiają tylko gry o statusie `completed`.
- Kategorie i wagi: Gameplay 35, Fabuła 25, Grafika 15, Audio 10, Regrywalność 10, Trudność 5.
- Punkty za miejsce `p` (1-based) w rankingu o `n` grach: `100 - (p-1)/(n-1) * 40`, a dla `n == 1` zawsze 100.
- Ocena końcowa to średnia ważona tylko z kategorii, w których gra ma miejsce (pominięte kategorie nie liczą się do wag), zaokrąglona do liczby całkowitej. Gra bez żadnej kategorii ma `score = NULL`.
- **Wstawienie gry przesuwa inne gry**, więc zmienia się `n` i punkty pozostałych. Każda zmiana rankingu (`PUT`) przelicza w jednej transakcji `score` wszystkich ukończonych gier tego użytkownika. Score przechowujemy w `user_games`, żeby sortowanie i półki nie liczyły go w locie.
- Zmiana statusu z `completed` na inny usuwa grę ze wszystkich rankingów, zagęszcza pozycje i przelicza oceny.
- Testy muszą pokryć co najmniej: `n=1`, wstawienie na początek, w środek i na koniec, pominięcie kategorii, usunięcie gry z rankingu i wpływ na oceny innych gier.

### Kolory grzbietu (`internal/spine`)

- Po dodaniu gry lub tytułu backend pobiera okładkę (timeout 5 s, limit 2 MB), dekoduje JPEG, PNG albo WebP (`golang.org/x/image/webp` jest dozwolony) i wyznacza dwa dominujące kolory przez prosty median cut lub k-means na pomniejszonym obrazie.
- `spine_color` to kolor dominujący, a `spine_accent` to drugi kolor o wyraźnie innej jasności lub odcieniu.
- `spine_ink` to `#FFFFFF` albo `#14121A`, w zależności od tego, co ma większy kontrast z `spine_color` (WCAG, min. 4,5:1).
- Jeśli pobranie okładki się nie uda, kolory biorą się deterministycznie z hasha tytułu, z palety 12 stonowanych barw.

### Integracje

- **IGDB:** token aplikacji z `https://id.twitch.tv/oauth2/token` (client credentials) trzymany w pamięci i odnawiany przed wygaśnięciem. Zapytania idą przez `POST https://api.igdb.com/v4/games`. **Tekst wyszukiwania escapuj** (w starej wersji szedł do zapytania bez escapowania). Okładki: `https://images.igdb.com/igdb/image/upload/t_cover_big/{image_id}.jpg`.
  - Wyszukiwanie: `search "…"; fields id,name,cover.image_id,first_release_date,genres.name,platforms.abbreviation,involved_companies.company.name,involved_companies.developer,total_rating; limit 12;`
  - Popularne teraz: premiery z ostatnich 30 dni z `rating_count > 0`, sortowane po ocenie, limit 12. Wynik cache'uj w pamięci na 6 h.
  - Polecenia: weź 3–5 najwyżej ocenionych ukończonych gier użytkownika i dla każdej pobierz `similar_games`. Odfiltruj gry, które już są w bibliotece. Każde polecenie ma pole `because` (tytuł i ocena gry źródłowej), bo UI pokazuje „Bo dałeś X ocenę Y”.
- **TMDB:** `search/multi` z `language=pl-PL` (filmy i seriale). Bez klucza endpoint zwraca pustą listę i flagę `tmdbConfigured: false`.
- **Jikan:** `v4/anime?q=…&limit=20&sfw=true`. Respektuj limit 3 żądań na sekundę.
- **MyMemory:** tłumaczenie opisu gry EN→PL, gdy `TRANSLATION_ENABLED=true`, z cache w `translation_cache`. UI zawsze pozwala pokazać oryginał.
- Każdy klient ma timeout, jest wołany z `context.Context` i zwraca błędy, które API tłumaczy na czytelne komunikaty (patrz „Teksty błędów”).

## API

Wszystkie odpowiedzi to JSON. Błędy mają postać `{"error": {"code": "igdb_not_configured", "message": "…"}}`.

```
GET    /api/health
GET    /api/config                 flagi + igdbConfigured, tmdbConfigured (publiczne)
POST   /api/auth/register          gdy AUTH_ENABLED
POST   /api/auth/login             ustawia cookie httpOnly, SameSite=Lax, Secure za HTTPS
POST   /api/auth/logout
GET    /api/me
PUT    /api/me/password

GET    /api/games/search?q=
GET    /api/games/popular
GET    /api/games/recommendations
GET    /api/games/{igdbId}         szczegóły + przetłumaczony opis + wpis użytkownika, jeśli jest

GET    /api/library                ?status=&sort=score|title|added
POST   /api/library                {igdbId, status, platform}
PATCH  /api/library/{id}           {status?, platform?, note?}
DELETE /api/library/{id}
GET    /api/library/{id}/rankings  miejsca gry we wszystkich kategoriach + n

GET    /api/rankings/{category}    uporządkowana lista gier
PUT    /api/rankings/{category}    {userGameIds: [...]}: pełna kolejność, zwraca przeliczone oceny

GET    /api/stats                  liczby per status, średnia, top 8

GET    /api/media                  ?type=&status=
GET    /api/media/search?q=&type=  TMDB + Jikan
POST   /api/media
PATCH  /api/media/{id}             {status?, score?, watchedEpisodes?, note?}
POST   /api/media/{id}/episode     +1 obejrzany odcinek; przy ostatnim status zmienia się na watched
DELETE /api/media/{id}
```

Endpointy `/api/media/*` istnieją tylko przy `MEDIA_ENABLED=true` (w przeciwnym razie 404). Walidacja wejścia odbywa się w handlerach, a każde zapytanie do bazy filtruje po `user_id` z kontekstu.

## Frontend: wygląd i ekrany

W katalogu `design/` jest 12 mockupów z redesignu (`*.dc.html`). To nie jest kod do skopiowania: format jest szablonowy (`{{…}}`, `<sc-for>`, `<sc-if>`), a dane są przykładowe. Bierz z nich układ, proporcje, kolory, typografię, teksty i zachowanie. Okładki w mockupach to geometryczne zaślepki; w aplikacji używasz prawdziwych okładek z IGDB i TMDB.

### Tokeny (`web/src/styles/tokens.css`)

```css
--bg: #1B1923;          /* tło */
--surface: #24212E;     /* panele, karty */
--surface-2: #2C2937;   /* aktywne elementy nawigacji */
--line: #34303F;        /* obramowania, deska półki */
--line-strong: #46415A;
--text: #EEEAF4;
--text-2: #CFC8DB;
--muted: #A49DB2;
--faint: #8A8398;
--accent: #7FD8BE;      /* główne akcje; tekst na nim: #14231E */
--score: #F2C14E;       /* oceny */
--st-in-progress: #7FD8BE;
--st-planned: #8EA8FF;
--st-completed: #F2C14E;
--st-on-hold: #F0935A;
--st-wishlist: #F28DBA;
--st-abandoned: #8A8398;
```

- **Typografia:** jedna rodzina, Archivo, a charakter daje oś szerokości. Duże nagłówki mają `font-stretch` 115–125% i wagę 800–900 z lekko ujemnym trackingiem. Tytuły na grzbietach mają 70–72% i wagę 700. Tekst bazowy to 15 px przy line-height 1.5. Liczby (oceny, liczniki) mają `font-variant-numeric: tabular-nums`.
- Nie pisz etykiet wersalikami. Ikony to inline SVG z obrysem, bez emoji.

### Komponent `Spine` (serce redesignu)

- Przycisk (`<button>` albo `<a>`) z grzbietem pudełka. Tło to `spine_color` z pionowym paskiem `spine_accent` na ostatnich 20% szerokości, a tekst ma kolor `spine_ink`.
- Na górze jest pasek 7 px w kolorze statusu. Pod nim skrót platformy (PC, PS5, XBOX, NSW). Tytuł biegnie pionowo, czytany od dołu do góry (`writing-mode: vertical-rl; transform: rotate(180deg)`), z wielokropkiem. Na dole jest ocena, jeśli istnieje.
- **Wysokość zależy od platformy:** PlayStation i Xbox mają 200 px, PC 186 px, Switch 166 px (Switch jest też węższy: 40 zamiast 46 px). Na telefonie wszystko skaluj do ok. 85%.
- Grzbiety mediów: seriale mają szerokość 52 px, anime 40 px, filmy 32 px.
- Stany: hover podnosi grzbiet o 4 px, a zaznaczony grzbiet (`aria-pressed="true"`) podnosi się o 14 px i ma miętową kreskę pod spodem. Przy `prefers-reduced-motion` nie ma animacji.
- Deska półki to pasek 12 px w `--line` z jaśniejszą krawędzią u góry. Rząd grzbietów przewija się poziomo, gdy się nie mieści.
- `aria-label` grzbietu: „{tytuł}, {status}, ocena {score}”.

### Ekrany (mockup w nawiasie)

1. **Odkryj** `/` (`Home.dc.html`): nagłówek „Grasz teraz w N gry” z odmianą liczebnika (1 grę, 2–4 gry, 5+ gier, z obsługą 12–14 i 22–24), okładki gier w trakcie i akcje „Ukończona” (otwiera ocenę w rankingu) oraz „Notatka”. Obok panel „Twój {rok}” z paskiem statusów i mini-półką top 5. Dalej „Popularne teraz” i „Może ci się spodobać”.
2. **Biblioteka** `/library` (`Main.dc.html`): sześć półek po jednej na status, w kolejności W trakcie, Planowane, Ukończone, Wstrzymane, Lista życzeń, Porzucone. Kliknięcie grzbietu otwiera boczny panel z grą (okładka, status i platforma, ocena z rozbiciem na kategorie jako rząd kresek z zaznaczonym miejscem, notatka, akcje). Przełącznik widoku Półka/Okładki i sortowanie (ocena, tytuł, data dodania).
3. **Karta gry** `/game/:igdbId` (`Game.dc.html`): status jako rząd przycisków, wybór platformy, sekcja „Twoja ocena” z sześcioma kategoriami (waga, miejsce „2. z 13”, punkty), notatka zapisywana automatycznie z debounce, opis z linkiem „Pokaż oryginał”.
4. **Ocena w rankingu** jako modal (`Ranking.dc.html`): sześć kroków-kategorii. W każdym kroku widać ranking ukończonych gier jako poziome grzbiety, a między nimi przyciski-szczeliny „Wstaw tutaj”. Obok ocena za kategorię i ocena końcowa przeliczane na żywo (ta sama formuła co w backendzie, w `web/src/lib/ranking.ts`, z testem Vitest zgodnym z testami Go). Jest „Cofnij”, „Pomiń tę kategorię” i „Dalej: {kategoria}”, a ostatni krok ma „Zapisz ocenę”. Cała obsługa działa z klawiatury. Drag & drop jest dodatkiem i nie może być jedyną drogą.
5. **Szukaj** `/search` (`Search.dc.html`): duże pole i zakładki „Gry” oraz „Filmy, seriale i anime”. Wyniki mają miniaturę, metadane i ocenę źródła, a obok „Na półce: {status}” albo przycisk dodawania. Wyszukiwanie z debounce 300 ms, a `q` jest w URL.
6. **Dodaj na półkę** jako modal (`AddGame.dc.html`): wybór jednej z sześciu półek w siatce 3×2, przy czym każda ma pasek koloru i krótki opis. Platformy ograniczone do tych z IGDB. Przycisk ma treść „Postaw na półce {status}”. Po zapisie widać potwierdzenie z grzbietem i „Cofnij”. Wybór „Ukończone” od razu otwiera ocenę w rankingu.
7. **Pierwsze uruchomienie** (`Welcome.dc.html`): pokazuj, gdy biblioteka jest pusta. Krok 1 to status IGDB (z `/api/config`), w wersji błędu z instrukcją, co dopisać do `.env`. Krok 2 to pole wyszukiwania.
8. **Filmy, seriale i anime** `/media` (`Media.dc.html`): filtr rodzaju, sekcja „Oglądasz” z paskiem odcinków i przyciskiem „+1 odcinek”, półki „Do obejrzenia” i „Obejrzane”. Ocenę mediów (0–100) wpisuje się ręcznie w panelu tytułu.
9. **Profil** `/profile` (`Profile.dc.html`): statystyki w jednym zdaniu, „Najwyższa półka” (top 8), „Konto” (zmiana hasła, wylogowanie, ukryte bez auth) i „Ten serwer” (flagi z `/api/config` z nazwami zmiennych, tylko do odczytu, oraz wersja aplikacji wstrzykiwana przy buildzie przez `-ldflags`).
10. **Logowanie i rejestracja** `/login` (`Login.dc.html`).
11. **Telefon** (`Mobile.dc.html`): poniżej 720 px górna nawigacja zamienia się w dolny pasek z pozycjami Odkryj, Biblioteka, Szukaj i Profil. Filtry statusów biblioteki to przewijane przyciski nad półką, a panel gry to dolny arkusz. Wszystkie pozostałe ekrany też muszą działać przy szerokości 390 px.

Nawigacja górna: wordmark „gamelogg” (szerokość 125%, waga 800, małe litery), Odkryj, Biblioteka, Filmy i seriale (tylko przy `MEDIA_ENABLED`), pole wyszukiwania, „Dodaj grę” i awatar.

### Dostępność (wymóg, nie dodatek)

- Prawdziwe `button`, `a` i `label`. Widoczny focus (2 px `--accent`). Cele dotyku co najmniej 44 px. Kontrast tekstu co najmniej 4,5:1.
- Modale łapią focus, zamykają się Escape i oddają focus elementowi, który je otworzył.
- Status nigdy nie jest przekazywany samym kolorem: zawsze towarzyszy mu tekst albo `aria-label`.

### Teksty błędów i pustych stanów

Piszemy konkretnie, bez przepraszania. Mówimy, co się stało i co zrobić. Przykłady:
- Brak kluczy IGDB: „Wyszukiwanie gier nie zadziała: brak IGDB_CLIENT_ID i IGDB_CLIENT_SECRET w .env. Dopisz oba klucze i uruchom ponownie kontener app.”
- IGDB nie odpowiada: „IGDB nie odpowiada. Spróbuj za chwilę.”
- Pusta półka w filtrze: „Na tej półce nic nie stoi.” i akcja „Dodaj grę”.
- Złe logowanie: „Email albo hasło się nie zgadzają. Sprawdź oba pola.”

## Testy

- **Go:** testy tabelaryczne dla `internal/ranking` i `internal/spine` oraz testy integracyjne `internal/store` na prawdziwym Postgresie (testcontainers-go albo serwis w compose, pomijane flagą `-short`). Handlery testuj przez `httptest` z atrapami klientów IGDB, TMDB i Jikan (interfejsy, bez wywołań sieciowych w testach).
- **Frontend:** Vitest dla `lib/ranking.ts`, odmiany liczebników i komponentu `Spine` (aria-label, wysokość per platforma).

## Etapy

0. **Szkielet:** struktura repo, config, połączenie z bazą, goose, `/api/health`, Vite z embedem w binarce, Dockerfile i compose, `.env.example`, Makefile (`make dev`, `make test`, `make build`).
1. **Auth i config:** rejestracja, logowanie, cookie, middleware, tryb `AUTH_ENABLED=false`, `/api/config`. Na froncie tokeny, font, layout z nawigacją (desktop i telefon) oraz ekrany logowania.
2. **Gry i biblioteka:** klient IGDB, wyszukiwanie, dodawanie, kolory grzbietu, `/api/library`. Na froncie Szukaj, modal dodawania, Biblioteka z półkami i panelem, ekran pierwszego uruchomienia.
3. **Rankingi:** `internal/ranking` z testami, endpointy rankingów, przeliczanie ocen, karta gry, modal oceny.
4. **Odkryj:** popularne, polecenia, statystyki, tłumaczenie opisów.
5. **Media:** TMDB, Jikan, `/api/media`, ekran mediów.
6. **Profil i domknięcie:** profil, zmiana hasła, przegląd dostępności i widoku mobilnego, README (szybki start, zdobycie kluczy IGDB i TMDB, przykład Caddy, backup Postgresa przez `pg_dump`).
