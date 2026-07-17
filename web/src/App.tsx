import { useEffect, useState } from 'react'
import { Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import {
  captureOAuthTokenFromHash,
  getToken,
  logout,
  me,
  setToken,
  type User,
} from './api'
import { AppShell } from './app_shell'
import { AuthScreen } from './auth_screen'
import './App.css'

type Gate = 'loading' | 'auth' | 'app'

export default function App() {
  const [gate, setGate] = useState<Gate>('loading')
  const [user, setUser] = useState<User | null>(null)
  const navigate = useNavigate()

  async function refreshSession() {
    captureOAuthTokenFromHash()
    const token = getToken()
    if (!token) {
      setUser(null)
      setGate('auth')
      return
    }
    try {
      const res = await me()
      setUser(res.user)
      setGate('app')
    } catch {
      setToken(null)
      setUser(null)
      setGate('auth')
    }
  }

  useEffect(() => {
    void refreshSession()
  }, [])

  async function onAuthed(token: string) {
    setToken(token)
    await refreshSession()
    navigate('/')
  }

  async function onLogout() {
    await logout()
    setUser(null)
    setGate('auth')
    navigate('/')
  }

  if (gate === 'loading') {
    return (
      <div className="app">
        <p className="lede">Loading…</p>
      </div>
    )
  }

  if (gate === 'auth') {
    return (
      <div className="app">
        <AuthScreen onAuthed={onAuthed} />
      </div>
    )
  }

  return (
    <Routes>
      <Route path="/" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="/orgs/:orgId" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="/orgs/:orgId/:section" element={<AppShell user={user} onLogout={onLogout} />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
