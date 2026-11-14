import { QueryCache, QueryClient, QueryClientProvider, QueryErrorResetBoundary } from '@tanstack/react-query'
import { Component, Suspense, type ReactNode } from 'react'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router'
import { isUnauthorized } from './lib/api'
import { meKey, useConfig } from './lib/session'
import { LoginPage } from './pages/LoginPage'
import { Placeholder } from './pages/Placeholder'
import { ProfilePage } from './pages/ProfilePage'
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
  return (
    <Placeholder
      title="Filmy, seriale i anime"
      lead="Półki z filmami, serialami i anime pojawią się w etapie 5."
    />
  )
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
        element: <Placeholder title="Odkryj" lead="Gry w trakcie, popularne premiery i polecenia pojawią się w etapie 4." />,
      },
      {
        path: 'library',
        element: <Placeholder title="Biblioteka" lead="Sześć półek z twoimi grami pojawi się w etapie 2." />,
      },
      {
        path: 'search',
        element: <Placeholder title="Czego szukasz?" lead="Wyszukiwanie w IGDB pojawi się w etapie 2." />,
      },
      {
        path: 'game/:igdbId',
        element: <Placeholder title="Karta gry" lead="Karta gry z oceną w rankingach pojawi się w etapie 3." />,
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
