import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import {
  authProviders,
  devLogin,
  googleAuthURL,
  type AuthProviders,
} from './api'

export function AuthScreen({ onAuthed }: { onAuthed: (token: string) => Promise<void> }) {
  const [providers, setProviders] = useState<AuthProviders | null>(null)
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void authProviders()
      .then(setProviders)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auth'))
  }, [])

  async function onDevLogin(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const res = await devLogin(email, name || email)
      await onAuthed(res.token)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sign-in failed')
    } finally {
      setBusy(false)
    }
  }

  const googleReady = providers?.google === true
  const loadingProviders = providers === null && !error

  return (
    <main className="page auth-page">
      <header>
        <p className="eyebrow">
          <Link to="/" className="auth-home-link">
            KYC
          </Link>
        </p>
        <h1>Sign in</h1>
        <p className="lede">Continue with Google to manage your organisations.</p>
      </header>

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      {loadingProviders ? (
        <p className="lede">Loading sign-in options…</p>
      ) : googleReady ? (
        <a className="google-btn" href={googleAuthURL()}>
          Continue with Google
        </a>
      ) : (
        <div className="auth-google-missing">
          <p className="google-btn google-btn-disabled" aria-disabled="true">
            Continue with Google
          </p>
          <p className="field-hint">
            Google OAuth is not configured. Set <code>GOOGLE_CLIENT_ID</code> and{' '}
            <code>GOOGLE_CLIENT_SECRET</code> on the API, then restart.
          </p>
        </div>
      )}

      {providers?.dev_login && (
        <form className="auth-form" onSubmit={onDevLogin}>
          <p className="lede">Local/dev sign-in (AUTH_DEV_LOGIN)</p>
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
            />
          </label>
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
          </label>
          <button type="submit" disabled={busy}>
            {busy ? 'Please wait…' : 'Dev sign in'}
          </button>
        </form>
      )}
    </main>
  )
}
