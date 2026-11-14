import { useState, type FormEvent } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router'
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
  const [q, setQ] = useState('')

  function submit(e: FormEvent) {
    e.preventDefault()
    const query = q.trim()
    navigate(query ? `/search?q=${encodeURIComponent(query)}` : '/search', { state: { seed: Date.now() } })
  }

  return (
    <form role="search" className={styles.search} onSubmit={submit}>
      <label className={styles.searchField}>
        <SearchIcon />
        <span className="sr-only">Szukaj gry</span>
        <input
          type="search"
          name="q"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Szukaj w IGDB…"
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
