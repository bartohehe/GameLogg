import { useEffect } from 'react'

interface Props {
  title: string
  lead: string
}

/** Tymczasowy ekran dla tras, które powstaną w kolejnych etapach. */
export function Placeholder({ title, lead }: Props) {
  useEffect(() => {
    document.title = `${title} · GameLogg`
  }, [title])

  return (
    <div className="page">
      <h1 className="page-title">{title}</h1>
      <p className="page-lead">{lead}</p>
    </div>
  )
}
