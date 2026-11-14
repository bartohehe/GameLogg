// Platformy przychodzą z backendu jako etykiety: "PC", "PS5", "Xbox Series", "Switch"…

export type PlatformFamily = 'playstation' | 'xbox' | 'pc' | 'switch' | 'other'

export function platformFamily(platform: string): PlatformFamily {
  const p = platform.trim().toLowerCase()
  if (p.startsWith('ps')) return 'playstation'
  if (p.startsWith('xbox')) return 'xbox'
  if (p.startsWith('switch')) return 'switch'
  if (p === 'pc' || p === 'mac' || p === 'linux') return 'pc'
  return 'other'
}

/** Skrót na grzbiecie: PC, PS5, XBOX, NSW. */
export function platformShort(platform: string): string {
  switch (platformFamily(platform)) {
    case 'playstation':
      return platform.replace(/\s+/g, '').toUpperCase().slice(0, 5)
    case 'xbox':
      return 'XBOX'
    case 'switch':
      return 'NSW'
    case 'pc':
      return 'PC'
    default:
      return platform.toUpperCase().slice(0, 4)
  }
}

export interface SpineSize {
  width: number
  height: number
}

/**
 * Wymiary grzbietu gry: PlayStation i Xbox 46×200, PC 46×186, Switch 40×166.
 * Na telefonie CSS skaluje je do ok. 85%.
 */
export function gameSpineSize(platform: string): SpineSize {
  switch (platformFamily(platform)) {
    case 'playstation':
    case 'xbox':
      return { width: 46, height: 200 }
    case 'switch':
      return { width: 40, height: 166 }
    default:
      return { width: 46, height: 186 }
  }
}
