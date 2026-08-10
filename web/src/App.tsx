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
import { DatabasesEdit } from './pages/databases/databases_edit'
import { DatabasesIndex } from './pages/databases/databases_index'
import { DatabasesNew } from './pages/databases/databases_new'
import { DatabasesShow } from './pages/databases/databases_show'
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
import { ActivityPage } from './pages/activity/activity_page'
import { APIKeysIndex } from './pages/api_keys/api_keys_index'
import { APIKeysNew } from './pages/api_keys/api_keys_new'
import { SettingsPage } from './pages/settings_page'
import { ProductFeaturesEdit } from './pages/product_features/product_features_edit'
import { ProductFeaturesIndex } from './pages/product_features/product_features_index'
import { ProductFeaturesNew } from './pages/product_features/product_features_new'
import { ProductFeaturesShow } from './pages/product_features/product_features_show'
import { ProductPlansEdit } from './pages/product_plans/product_plans_edit'
import { ProductPlansIndex } from './pages/product_plans/product_plans_index'
import { ProductPlansNew } from './pages/product_plans/product_plans_new'
import { ProductPlansShow } from './pages/product_plans/product_plans_show'
import { UsersEdit } from './pages/users/users_edit'
import { UsersIndex } from './pages/users/users_index'
import { AppAccessPage } from './pages/app_access/app_access_page'
import { UsersNew } from './pages/users/users_new'
import { UsersShow } from './pages/users/users_show'
import { WebhooksEdit } from './pages/webhooks/webhooks_edit'
import { WebhooksIndex } from './pages/webhooks/webhooks_index'
import { WebhooksNew } from './pages/webhooks/webhooks_new'
import { WebhooksShow } from './pages/webhooks/webhooks_show'
import { InboundWebhooksEdit } from './pages/inbound_webhooks/inbound_webhooks_edit'
import { InboundWebhooksIndex } from './pages/inbound_webhooks/inbound_webhooks_index'
import { InboundWebhooksNew } from './pages/inbound_webhooks/inbound_webhooks_new'
import { InboundWebhooksShow } from './pages/inbound_webhooks/inbound_webhooks_show'
import { DocsConceptPage, DocsConceptsIndex } from './pages/docs/docs_concepts'
import { DocsIntegrationApiPage, DocsOperatorApiPage } from './pages/docs/docs_api'
import { DocsLayout } from './pages/docs/docs_layout'
import { DocsVariablesPage } from './pages/docs/docs_variables'
import { initTheme } from './theme'
import './App.css'

initTheme()

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
        path="/docs"
        element={
          <div className="app public-docs">
            <DocsLayout publicChrome user={user} />
          </div>
        }
      >
        <Route index element={<DocsConceptsIndex />} />
        <Route path="concepts/:slug" element={<DocsConceptPage />} />
        <Route path="api" element={<DocsIntegrationApiPage />} />
        <Route path="api/operator" element={<DocsOperatorApiPage />} />
        <Route path="variables" element={<DocsVariablesPage />} />
        <Route path="operator" element={<Navigate to="api/operator" replace />} />
      </Route>
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
        {/* Operator roles UI hidden; seeded roles still used when inviting members. */}
        <Route path="roles/*" element={<Navigate to="../members" replace />} />
        <Route path="users" element={<UsersIndex />} />
        <Route path="customer-access" element={<AppAccessPage />} />
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
        <Route path="databases" element={<DatabasesIndex />} />
        <Route path="databases/new" element={<DatabasesNew />} />
        <Route path="databases/:id" element={<DatabasesShow />} />
        <Route path="databases/:id/edit" element={<DatabasesEdit />} />
        <Route path="webhooks" element={<WebhooksIndex />} />
        <Route path="webhooks/new" element={<WebhooksNew />} />
        <Route path="webhooks/:id" element={<WebhooksShow />} />
        <Route path="webhooks/:id/edit" element={<WebhooksEdit />} />
        <Route path="inbound-webhooks" element={<InboundWebhooksIndex />} />
        <Route path="inbound-webhooks/new" element={<InboundWebhooksNew />} />
        <Route path="inbound-webhooks/:id" element={<InboundWebhooksShow />} />
        <Route path="inbound-webhooks/:id/edit" element={<InboundWebhooksEdit />} />
        <Route path="automations" element={<AutomationsIndex />} />
        <Route path="automations/new" element={<AutomationsNew />} />
        <Route path="automations/:id" element={<AutomationsShow />} />
        <Route path="automations/:id/edit" element={<AutomationsEdit />} />
        <Route path="product-features" element={<ProductFeaturesIndex />} />
        <Route path="product-features/new" element={<ProductFeaturesNew />} />
        <Route path="product-features/:id" element={<ProductFeaturesShow />} />
        <Route path="product-features/:id/edit" element={<ProductFeaturesEdit />} />
        <Route path="product-plans" element={<ProductPlansIndex />} />
        <Route path="product-plans/new" element={<ProductPlansNew />} />
        <Route path="product-plans/:id" element={<ProductPlansShow />} />
        <Route path="product-plans/:id/edit" element={<ProductPlansEdit />} />
        <Route path="branding" element={<BrandingPage />} />
        <Route path="billing" element={<BillingPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="api-keys" element={<APIKeysIndex />} />
        <Route path="api-keys/new" element={<APIKeysNew />} />
        <Route path="activity" element={<ActivityPage />} />
        <Route path="docs" element={<DocsLayout />}>
          <Route index element={<DocsConceptsIndex />} />
          <Route path="concepts/:slug" element={<DocsConceptPage />} />
          <Route path="api" element={<DocsIntegrationApiPage />} />
          <Route path="api/operator" element={<DocsOperatorApiPage />} />
          <Route path="variables" element={<DocsVariablesPage />} />
          <Route path="operator" element={<Navigate to="api/operator" replace />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
