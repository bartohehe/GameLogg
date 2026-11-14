import { QueryCache, QueryClient, QueryClientProvider, QueryErrorResetBoundary } from '@tanstack/react-query'
import { Component, Suspense, type ReactNode } from 'react'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router'
import { isUnauthorized } from './lib/api'
import { meKey, useConfig } from './lib/session'
import { DiscoverPage } from './pages/DiscoverPage'
import { GamePage } from './pages/GamePage'
import { LibraryPage } from './pages/LibraryPage'
import { LoginPage } from './pages/LoginPage'
import { MediaPage } from './pages/MediaPage'
import { Placeholder } from './pages/Placeholder'
import { ProfilePage } from './pages/ProfilePage'
import { SearchPage } from './pages/SearchPage'
import { ErrorScreen } from './routes/ErrorScreen'
import { AuthOnly, RequireUser } from './routes/guards'

const queryClient: QueryClient = new QueryClient({
  queryCache: new QueryCache({
    // Wygasła sesja w dowolnym zapytaniu: odśwież /me, a strażnik przekieruje na logowanie.
    onError: (err, query) => {
      if (isUnauthorized(err) && query.queryKey[0] !== meKey[0]) {
        void queryClient.invalidateQueries({ queryKey: meKey })
      }
    },
  }),
  defaultOptions: {
    queries: { refetchOnWindowFocus: false },
  },
})

function MediaRoute() {
  const config = useConfig()
  if (!config.mediaEnabled) return <Navigate to="/" replace />
  return <MediaPage />
}

const router = createBrowserRouter([
  {
    path: '/login',
    element: (
      <AuthOnly>
        <LoginPage />
      </AuthOnly>
    ),
  },
  {
    element: <RequireUser />,
    children: [
      {
        index: true,
        element: <DiscoverPage />,
      },
      {
        path: 'library',
        element: <LibraryPage />,
      },
      {
        path: 'search',
        element: <SearchPage />,
      },
      {
        path: 'game/:igdbId',
        element: <GamePage />,
      },
      { path: 'media', element: <MediaRoute /> },
      { path: 'profile', element: <ProfilePage /> },
      {
        path: '*',
        element: <Placeholder title="Nie ma takiej strony" lead="Sprawdź adres albo wróć do biblioteki." />,
      },
    ],
  },
])

// Łapie błąd ładowania /api/config (Suspense), np. gdy kontener app nie działa.
class StartupBoundary extends Component<{ children: ReactNode; onReset: () => void }, { error: Error | null }> {
  state = { error: null as Error | null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  render() {
    if (this.state.error) {
      return (
        <ErrorScreen
          message={this.state.error.message}
          onRetry={() => {
            this.props.onReset()
            this.setState({ error: null })
          }}
        />
      )
    }
    return this.props.children
  }
}

function ConfigGate({ children }: { children: ReactNode }) {
  useConfig()
  return children
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <QueryErrorResetBoundary>
        {({ reset }) => (
          <StartupBoundary onReset={reset}>
            <Suspense fallback={null}>
              <ConfigGate>
                <RouterProvider router={router} />
              </ConfigGate>
            </Suspense>
          </StartupBoundary>
        )}
      </QueryErrorResetBoundary>
    </QueryClientProvider>
  )
}
