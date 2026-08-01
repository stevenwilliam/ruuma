// Four sign-in methods (D24). Guest checkout does not exist (D12), and a
// verified phone is required before the first order (BR-2.7.4) — so the phone
// step is offered right after signing in.

import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, tokens, type Session } from '../../lib/api'
import { Button, Card, ErrorNote, Field, TextInput } from '../../components/ui'
import { t } from '../../i18n'

type Mode = 'signin' | 'signup' | 'phone'

export default function AuthPage() {
  const copy = t()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const next = params.get('next') ?? '/menu'

  const [mode, setMode] = useState<Mode>('signin')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [otp, setOtp] = useState('')
  const [otpSent, setOtpSent] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function run(fn: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const signIn = () =>
    run(async () => {
      const session = await api.post<Session>('/auth/login', { email, password })
      tokens.set(session.access_token, session.refresh_token)
      navigate(next)
    })

  const signUp = () =>
    run(async () => {
      await api.post('/auth/register', { email, password, full_name: fullName })
      setNotice(copy.auth.checkEmail)
    })

  const sendOtp = () =>
    run(async () => {
      await api.post('/otp/request', { phone, purpose: 'login' })
      setOtpSent(true)
    })

  const verifyOtp = () =>
    run(async () => {
      const session = await api.post<Session>('/otp/verify', {
        phone,
        code: otp,
        purpose: 'login',
      })
      tokens.set(session.access_token, session.refresh_token)
      navigate(next)
    })

  return (
    <div className="mx-auto flex max-w-md flex-col gap-4">
      <h1 className="font-display text-2xl font-bold">
        {mode === 'signup' ? copy.auth.signUp : copy.auth.signIn}
      </h1>

      <div className="flex gap-2" role="tablist">
        <Button variant={mode === 'signin' ? 'primary' : 'secondary'} onClick={() => setMode('signin')}>
          {copy.auth.email}
        </Button>
        <Button variant={mode === 'phone' ? 'primary' : 'secondary'} onClick={() => setMode('phone')}>
          {copy.auth.phone}
        </Button>
      </div>

      {notice && <Card>{notice}</Card>}
      {error && <ErrorNote>{error}</ErrorNote>}

      {mode !== 'phone' && (
        <Card className="flex flex-col gap-3">
          {mode === 'signup' && (
            <Field label={copy.checkout.contactName}>
              {(id) => (
                <TextInput id={id} value={fullName} onChange={(e) => setFullName(e.target.value)} />
              )}
            </Field>
          )}
          <Field label={copy.auth.email}>
            {(id) => (
              <TextInput
                id={id}
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            )}
          </Field>
          <Field label={copy.auth.password} hint="minimum 12">
            {(id) => (
              <TextInput
                id={id}
                type="password"
                autoComplete={mode === 'signup' ? 'new-password' : 'current-password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            )}
          </Field>

          <Button disabled={busy} onClick={mode === 'signup' ? signUp : signIn}>
            {mode === 'signup' ? copy.auth.signUp : copy.auth.signIn}
          </Button>

          <button
            type="button"
            className="text-sm text-primary underline underline-offset-4"
            onClick={() => setMode(mode === 'signup' ? 'signin' : 'signup')}
          >
            {mode === 'signup' ? copy.auth.haveAccount : copy.auth.needAccount}
          </button>
        </Card>
      )}

      {mode === 'phone' && (
        <Card className="flex flex-col gap-3">
          <Field label={copy.auth.phone} hint="08… / +62…">
            {(id) => (
              <TextInput
                id={id}
                inputMode="tel"
                autoComplete="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
              />
            )}
          </Field>
          {otpSent && (
            <Field label={copy.auth.otp}>
              {(id) => (
                <TextInput
                  id={id}
                  inputMode="numeric"
                  maxLength={6}
                  value={otp}
                  onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                />
              )}
            </Field>
          )}
          <Button disabled={busy} onClick={otpSent ? verifyOtp : sendOtp}>
            {otpSent ? copy.auth.verify : copy.auth.sendOtp}
          </Button>
        </Card>
      )}

      <div className="flex flex-col gap-2">
        <p className="text-center text-xs text-muted">{copy.auth.orContinueWith}</p>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            className="flex-1"
            onClick={() =>
              run(async () => {
                const res = await api.post<{ authorize_url: string }>('/auth/oauth/google/start')
                window.location.href = res.authorize_url
              })
            }
          >
            {copy.auth.google}
          </Button>
          <Button
            variant="secondary"
            className="flex-1"
            onClick={() =>
              run(async () => {
                const res = await api.post<{ authorize_url: string }>('/auth/oauth/instagram/start')
                window.location.href = res.authorize_url
              })
            }
          >
            {copy.auth.instagram}
          </Button>
        </div>
      </div>
    </div>
  )
}
