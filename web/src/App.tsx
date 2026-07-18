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
import { AttributesEdit } from './pages/attributes/attributes_edit'
import { AttributesIndex } from './pages/attributes/attributes_index'
import { AttributesNew } from './pages/attributes/attributes_new'
import { AttributesShow } from './pages/attributes/attributes_show'
import { BillingPage } from './pages/billing_page'
import { BrandingPage } from './pages/branding_page'
import { AutomationsEdit } from './pages/automations/automations_edit'
import { AutomationsIndex } from './pages/automations/automations_index'
import { AutomationsNew } from './pages/automations/automations_new'
import { AutomationsShow } from './pages/automations/automations_show'
import { EmailTemplatesEdit } from './pages/email_templates/email_templates_edit'
import { EmailTemplatesIndex } from './pages/email_templates/email_templates_index'
import { EmailTemplatesNew } from './pages/email_templates/email_templates_new'
import { EmailTemplatesShow } from './pages/email_templates/email_templates_show'
import { LandingPage } from './pages/landing_page'
import { MembersEdit } from './pages/members/members_edit'
import { MembersIndex } from './pages/members/members_index'
import { MembersNew } from './pages/members/members_new'
import { MembersShow } from './pages/members/members_show'
import { OverviewPage } from './pages/overview_page'
import { RolesEdit } from './pages/roles/roles_edit'
import { RolesIndex } from './pages/roles/roles_index'
import { RolesNew } from './pages/roles/roles_new'
import { RolesShow } from './pages/roles/roles_show'
import { UsersEdit } from './pages/users/users_edit'
import { UsersIndex } from './pages/users/users_index'
import { UsersNew } from './pages/users/users_new'
import { UsersShow } from './pages/users/users_show'
import './App.css'

type Gate = 'loading' | 'ready'

export default function App() {
  const [gate, setGate] = useState<Gate>('loading')
  const [user, setUser] = useState<User | null>(null)
  const navigate = useNavigate()

  async function refreshSession() {
    captureOAuthTokenFromHash()
    const token = getToken()
    if (!token) {
      setUser(null)
      setGate('ready')
      return
    }
    try {
      const res = await me()
      setUser(res.user)
      setGate('ready')
    } catch {
      setToken(null)
      setUser(null)
      setGate('ready')
    }
  }

  useEffect(() => {
    void refreshSession()
  }, [])

  async function onAuthed(token: string) {
    setToken(token)
    await refreshSession()
    navigate('/app')
  }

  async function onLogout() {
    await logout()
    setUser(null)
    setGate('ready')
    navigate('/', { replace: true })
  }

  if (gate === 'loading') {
    return (
      <div className="app">
        <p className="lede">Loading…</p>
      </div>
    )
  }

  return (
    <Routes>
      <Route path="/" element={<LandingPage user={user} />} />
      <Route
        path="/login"
        element={
          user ? (
            <Navigate to="/app" replace />
          ) : (
            <div className="app">
              <AuthScreen onAuthed={onAuthed} />
            </div>
          )
        }
      />
      <Route
        path="/app"
        element={
          user ? <AppShell user={user} onLogout={onLogout} /> : <Navigate to="/login" replace />
        }
      />
      <Route
        path="/orgs/:orgId"
        element={
          user ? <AppShell user={user} onLogout={onLogout} /> : <Navigate to="/login" replace />
        }
      >
        <Route index element={<OverviewPage />} />
        <Route path="members" element={<MembersIndex />} />
        <Route path="members/new" element={<MembersNew />} />
        <Route path="members/:id" element={<MembersShow />} />
        <Route path="members/:id/edit" element={<MembersEdit />} />
        <Route path="roles" element={<RolesIndex />} />
        <Route path="roles/new" element={<RolesNew />} />
        <Route path="roles/:id" element={<RolesShow />} />
        <Route path="roles/:id/edit" element={<RolesEdit />} />
        <Route path="users" element={<UsersIndex />} />
        <Route path="users/new" element={<UsersNew />} />
        <Route path="users/:id" element={<UsersShow />} />
        <Route path="users/:id/edit" element={<UsersEdit />} />
        <Route path="attributes" element={<AttributesIndex />} />
        <Route path="attributes/new" element={<AttributesNew />} />
        <Route path="attributes/:id" element={<AttributesShow />} />
        <Route path="attributes/:id/edit" element={<AttributesEdit />} />
        <Route path="email-templates" element={<EmailTemplatesIndex />} />
        <Route path="email-templates/new" element={<EmailTemplatesNew />} />
        <Route path="email-templates/:id" element={<EmailTemplatesShow />} />
        <Route path="email-templates/:id/edit" element={<EmailTemplatesEdit />} />
        <Route path="automations" element={<AutomationsIndex />} />
        <Route path="automations/new" element={<AutomationsNew />} />
        <Route path="automations/:id" element={<AutomationsShow />} />
        <Route path="automations/:id/edit" element={<AutomationsEdit />} />
        <Route path="branding" element={<BrandingPage />} />
        <Route path="billing" element={<BillingPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
