// Ikony z mockupów: inline SVG z obrysem, kolor z currentColor.
// Zawsze dekoracyjne (aria-hidden): znaczenie niesie tekst albo aria-label rodzica.

interface IconProps {
  size?: number
}

const base = (size: number) => ({
  width: size,
  height: size,
  fill: 'none',
  stroke: 'currentColor',
  'aria-hidden': true,
  focusable: false,
})

export function SearchIcon({ size = 16 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 16 16" strokeWidth={1.6}>
      <circle cx="7" cy="7" r="5" />
      <path d="M11 11l3.5 3.5" />
    </svg>
  )
}

export function DiscoverIcon({ size = 20 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 20 20" strokeWidth={1.6}>
      <path d="M10 2l2.4 5.6L18 10l-5.6 2.4L10 18l-2.4-5.6L2 10l5.6-2.4z" />
    </svg>
  )
}

export function LibraryIcon({ size = 20 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 20 20" strokeWidth={1.6}>
      <path d="M3 3v13M7 5v11M11 3v13M14 6l3 10M2 17h16" />
    </svg>
  )
}

export function ProfileIcon({ size = 20 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 20 20" strokeWidth={1.6}>
      <circle cx="10" cy="7" r="3.5" />
      <path d="M3 18c0-3.9 3.1-7 7-7s7 3.1 7 7" />
    </svg>
  )
}

export function PlusIcon({ size = 16 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 16 16" strokeWidth={2}>
      <path d="M8 2v12M2 8h12" />
    </svg>
  )
}

export function CloseIcon({ size = 14 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 14 14" strokeWidth={1.8}>
      <path d="M2 2l10 10M12 2L2 12" />
    </svg>
  )
}

export function BackIcon({ size = 16 }: IconProps) {
  return (
    <svg {...base(size)} viewBox="0 0 16 16" strokeWidth={1.8}>
      <path d="M10 3L5 8l5 5" />
    </svg>
  )
}
