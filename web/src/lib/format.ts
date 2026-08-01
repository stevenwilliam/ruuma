// Money is an integer count of rupiah end to end (BR-1.1.1/4); the UI only ever
// formats it for display and never does arithmetic on a formatted string.

export function rupiah(value: number): string {
  const sign = value < 0 ? '-' : ''
  const digits = Math.abs(Math.trunc(value)).toString()
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${sign}Rp ${grouped}`
}

export function timeOfDay(iso: string, timeZone = 'Asia/Jakarta'): string {
  return new Intl.DateTimeFormat('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone,
  }).format(new Date(iso))
}

export function longDate(iso: string, locale = 'id-ID', timeZone = 'Asia/Jakarta'): string {
  return new Intl.DateTimeFormat(locale, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    timeZone,
  }).format(new Date(iso))
}

export function shortDate(iso: string, locale = 'id-ID', timeZone = 'Asia/Jakarta'): string {
  return new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    timeZone,
  }).format(new Date(iso))
}

export function slotLabel(startsAt: string, endsAt: string, timeZone = 'Asia/Jakarta'): string {
  return `${timeOfDay(startsAt, timeZone)}–${timeOfDay(endsAt, timeZone)}`
}

export function relativeMinutes(iso: string): number {
  return Math.round((Date.now() - new Date(iso).getTime()) / 60000)
}
