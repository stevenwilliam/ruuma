// Photo credits. Most dish photography is CC BY or CC BY-SA from Wikimedia
// Commons, and those licences oblige us to name the photographer and the
// licence wherever the work is published (D31). This page is that obligation,
// not decoration — it is linked from the footer of every page.
//
// When ruuma's own photography replaces these, entries disappear from
// credits.json and this page shrinks to nothing on its own.

import { useMemo, useState } from 'react'
import credits from '../../credits.json'
import { SearchBox } from '../../components/SearchBox'
import { Card, EmptyState } from '../../components/ui'
import { t } from '../../i18n'

type Credit = {
  sku: string
  file: string
  author: string
  licence: string
  licence_url: string
  source_url: string
}

export default function CreditsPage() {
  const copy = t()
  const [query, setQuery] = useState('')
  const rows = credits as Credit[]

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return rows
    return rows.filter((c) =>
      [c.sku, c.file, c.author, c.licence].some((v) => v.toLowerCase().includes(q)),
    )
  }, [rows, query])

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-2xl font-bold">{copy.credits.title}</h1>
        <p className="text-sm text-muted">{copy.credits.intro}</p>
      </header>

      {/* BR-1.5.1: every list has a search box. */}
      <SearchBox value={query} onChange={setQuery} />

      {filtered.length === 0 && <EmptyState>{copy.common.empty}</EmptyState>}

      <ul className="flex flex-col gap-2">
        {filtered.map((c) => (
          <li key={c.sku}>
            <Card className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <img
                src={`/dish/${c.sku}.jpg`}
                alt=""
                loading="lazy"
                width={160}
                height={120}
                className="h-16 w-24 shrink-0 self-start rounded-lg object-cover"
              />
              <div className="min-w-0 text-sm">
                <p className="font-medium">{c.sku}</p>
                <p className="truncate text-muted">{c.file}</p>
                <p className="text-muted">
                  {copy.credits.by} {c.author} —{' '}
                  {c.licence_url ? (
                    <a
                      href={c.licence_url}
                      rel="noreferrer nofollow"
                      target="_blank"
                      className="text-primary-ink underline underline-offset-2"
                    >
                      {c.licence}
                    </a>
                  ) : (
                    c.licence
                  )}
                </p>
              </div>
              <a
                href={c.source_url}
                rel="noreferrer nofollow"
                target="_blank"
                className="text-sm font-medium text-primary-ink underline underline-offset-4 sm:ml-auto"
              >
                {copy.credits.source}
              </a>
            </Card>
          </li>
        ))}
      </ul>
    </div>
  )
}
