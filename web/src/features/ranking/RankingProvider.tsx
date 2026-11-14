import { useState, type ReactNode } from 'react'
import { RankingContext, type RankingTarget } from './context'
import { RankingModal } from './RankingModal'

export function RankingProvider({ children }: { children: ReactNode }) {
  const [target, setTarget] = useState<RankingTarget | null>(null)
  return (
    <RankingContext.Provider value={setTarget}>
      {children}
      {target && <RankingModal key={target.entryId} target={target} onClose={() => setTarget(null)} />}
    </RankingContext.Provider>
  )
}
