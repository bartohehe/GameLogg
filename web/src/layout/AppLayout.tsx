import { useState, type FormEvent } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router'
import { DiscoverIcon, LibraryIcon, ProfileIcon, SearchIcon } from '../components/icons'
import { RankingProvider } from '../features/ranking/RankingProvider'
import type { User } from '../lib/api'
import { useConfig } from '../lib/session'
import styles from './AppLayout.module.css'

function initial(name: string) {
  return (name.trim()[0] ?? '?').toLocaleUpperCase('pl-PL')
}

function HeaderSearch() {
  const navigate = useNavigate()
  const location = useLocation()
  const [q, setQ] = useState('')
  // Na ekranie mediów pole szuka filmów, seriali i anime zamiast gier.
  const media = location.pathname.startsWith('/media')

  function submit(e: FormEvent) {
    e.preventDefault()
    const params = new URLSearchParams()
    const query = q.trim()
    if (query) params.set('q', query)
    if (media) params.set('tab', 'media')
    const qs = params.toString()
    navigate(qs ? `/search?${qs}` : '/search', { state: { seed: Date.now() } })
  }

  return (
    <form role="search" className={styles.search} onSubmit={submit}>
      <label className={styles.searchField}>
        <SearchIcon />
        <span className="sr-only">{media ? 'Szukaj filmów, seriali i anime' : 'Szukaj gry'}</span>
        <input
          type="search"
          name="q"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={media ? 'Szukaj filmów i seriali…' : 'Szukaj w IGDB…'}
          autoComplete="off"
        />
      </label>
    </form>
  )
}

export function AppLayout({ user }: { user: User }) {
  const config = useConfig()
  const navClass = ({ isActive }: { isActive: boolean }) => (isActive ? `${styles.navLink} ${styles.active}` : styles.navLink)
  const tabClass = ({ isActive }: { isActive: boolean }) => (isActive ? `${styles.tab} ${styles.active}` : styles.tab)

  return (
    <div className={styles.shell}>
      <a href="#main" className={styles.skip}>
        Przejdź do treści
      </a>

      <header className={styles.header}>
        <Link to="/" className={styles.wordmark}>
          gamelogg
        </Link>
        <nav aria-label="Główna" className={styles.nav}>
          <NavLink to="/" end className={navClass}>
            Odkryj
          </NavLink>
          <NavLink to="/library" className={navClass}>
            Biblioteka
          </NavLink>
          {config.mediaEnabled && (
            <NavLink to="/media" className={navClass}>
              Filmy i seriale
            </NavLink>
          )}
        </nav>
        <HeaderSearch />
        <Link to="/search" className="btn btn-primary">
          Dodaj grę
        </Link>
        <NavLink to="/profile" className={styles.avatar} aria-label={`Profil: ${user.username}`}>
          {initial(user.username)}
        </NavLink>
      </header>

      <main id="main" tabIndex={-1} className={styles.main}>
        <RankingProvider>
          <Outlet />
        </RankingProvider>
      </main>

      <nav aria-label="Główna" className={styles.tabbar}>
        <NavLink to="/" end className={tabClass}>
          <DiscoverIcon />
          Odkryj
        </NavLink>
        <NavLink to="/library" className={tabClass}>
          <LibraryIcon />
          Biblioteka
        </NavLink>
        <NavLink to="/search" className={tabClass}>
          <SearchIcon size={20} />
          Szukaj
        </NavLink>
        <NavLink to="/profile" className={tabClass}>
          <ProfileIcon />
          Profil
        </NavLink>
      </nav>
    </div>
  )
}
