// A catch-all, because there was not one.
//
// nginx serves index.html for any path, and React Router with no matching
// route rendered the layout around nothing at all — so a mistyped URL, or the
// /verify-email link that had no route, produced a header, a footer and an
// empty middle. That reads as a broken site rather than a wrong address.

import { Link } from 'react-router-dom'
import { Card } from '../../components/ui'
import { useSeo, useNoIndex } from '../../lib/seo'
import { t } from '../../i18n'

export default function NotFoundPage() {
  const copy = t()

  useSeo(copy.notFound.title)
  // A 404 that gets indexed competes with the real pages for the same query.
  useNoIndex()

  return (
    <div className="mx-auto flex max-w-md flex-col gap-5">
      <header className="flex flex-col gap-1.5">
        <span className="eyebrow">404</span>
        <h1 className="font-display text-3xl font-semibold leading-tight">
          {copy.notFound.title}
        </h1>
      </header>

      <Card className="flex flex-col gap-3">
        <p className="text-sm text-muted">{copy.notFound.body}</p>
        <Link
          to="/menu"
          className="w-fit text-sm font-medium text-primary-ink underline underline-offset-4"
        >
          {copy.notFound.home}
        </Link>
      </Card>
    </div>
  )
}
