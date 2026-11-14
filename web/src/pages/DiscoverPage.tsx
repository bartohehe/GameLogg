import { Welcome } from '../features/Welcome'
import { useLibrary } from '../lib/games'
import { Placeholder } from './Placeholder'

// Wersja z etapu 2: przy pustej bibliotece ekran pierwszego uruchomienia.
// Gry w trakcie, „Twój rok”, popularne i polecenia dochodzą w etapie 4.
export function DiscoverPage() {
  const lib = useLibrary()
  if (lib.isPending) return <div className="page" aria-busy="true" />
  if (lib.isSuccess && lib.data.length === 0) return <Welcome />
  return <Placeholder title="Odkryj" lead="Gry w trakcie, popularne premiery i polecenia pojawią się w etapie 4." />
}
