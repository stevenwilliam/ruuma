// The page a registration email links to.
//
// It did not exist. authsvc builds the link as {baseURL}/verify-email?token=…,
// which is a *frontend* path, while the API route is
// /api/v1/auth/verify-email — so every verification email ever sent pointed at
// a route the SPA does not declare, and landed on a blank page. Email
// verification could not have worked for anyone.
//
// The link could instead have pointed straight at the API, but then a customer
// clicking a link in their inbox is shown raw JSON. This page calls the
// endpoint and says what happened in words.

import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../../lib/api'
import { Card, ErrorNote, Spinner } from '../../components/ui'
import { useSeo, useNoIndex } from '../../lib/seo'
import { t } from '../../i18n'

type State = 'checking' | 'done' | 'failed' | 'no-token'

export default function VerifyEmailPage() {
  const copy = t()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''

  const [state, setState] = useState<State>(token ? 'checking' : 'no-token')
  const [detail, setDetail] = useState('')

  useSeo(copy.verifyEmail.title)
  // A one-shot token in the query string must never reach an index.
  useNoIndex()

  // The token is single-use, so a second call always fails. React 18 mounts
  // effects twice in StrictMode, which would have made the first verification
  // succeed and the second one paint an error over it.
  const sent = useRef(false)

  useEffect(() => {
    if (!token || sent.current) return
    sent.current = true

    api
      .get(`/auth/verify-email?token=${encodeURIComponent(token)}`)
      .then(() => setState('done'))
      .catch((err: Error) => {
        setState('failed')
        setDetail(err.message)
      })
  }, [token])

  return (
    <div className="mx-auto flex max-w-md flex-col gap-5">
      <header className="flex flex-col gap-1.5">
        <span className="eyebrow">{copy.brand}</span>
        <h1 className="font-display text-3xl font-semibold leading-tight">
          {copy.verifyEmail.title}
        </h1>
      </header>

      {state === 'checking' && <Spinner label={copy.verifyEmail.checking} />}

      {state === 'no-token' && <ErrorNote>{copy.verifyEmail.missingToken}</ErrorNote>}

      {state === 'done' && (
        <Card className="flex flex-col gap-3">
          <p className="text-sm">{copy.verifyEmail.success}</p>
          <Link
            to="/signin"
            className="w-fit text-sm font-medium text-primary-ink underline underline-offset-4"
          >
            {copy.common.signIn}
          </Link>
        </Card>
      )}

      {state === 'failed' && (
        <div className="flex flex-col gap-3">
          <ErrorNote>{copy.verifyEmail.failed}</ErrorNote>
          <Card className="flex flex-col gap-3">
            <p className="text-sm text-muted">{copy.verifyEmail.expiredHint}</p>
            {/* The server's message is shown too: "already verified" and
                "expired" are different problems and the customer can act on
                the difference. */}
            {detail && <p className="text-xs text-muted">{detail}</p>}
            <Link
              to="/signin"
              className="w-fit text-sm font-medium text-primary-ink underline underline-offset-4"
            >
              {copy.common.signIn}
            </Link>
          </Card>
        </div>
      )}
    </div>
  )
}
