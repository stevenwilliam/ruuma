// ID is the default; EN is the alternate (D9). Copy lives here, never inline in
// a component, so a translator never has to read TSX.

import { id } from './id'
import { en } from './en'

export type Lang = 'id' | 'en'
export type Catalog = typeof id

const catalogs: Record<Lang, Catalog> = { id, en }

const LANG_KEY = 'ruuma.lang'

export function currentLang(): Lang {
  const stored = localStorage.getItem(LANG_KEY)
  return stored === 'en' ? 'en' : 'id'
}

export function setLang(lang: Lang) {
  localStorage.setItem(LANG_KEY, lang)
  document.documentElement.lang = lang
}

export function t(): Catalog {
  return catalogs[currentLang()]
}

// localeName picks the right language field on an API object without every
// component repeating the ternary.
export function localeName(obj: { name_id: string; name_en: string }): string {
  return currentLang() === 'en' ? obj.name_en : obj.name_id
}

export function localeDesc(obj: { description_id: string; description_en: string }): string {
  return currentLang() === 'en' ? obj.description_en : obj.description_id
}
