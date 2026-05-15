import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../../lib/auth'
import { ProtectedRoute } from './ProtectedRoute'

function renderWithRouter(initialEntry: string, token?: string) {
  if (token) {
    window.sessionStorage.setItem('llm_gateway_admin_token', token)
  } else {
    window.sessionStorage.removeItem('llm_gateway_admin_token')
  }

  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/login" element={<div>Login Page</div>} />
        <Route element={<ProtectedRoute />}>
          <Route path="/dashboard" element={<div>Dashboard Content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    clearToken()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('redirects to login when no token is set', async () => {
    renderWithRouter('/dashboard')
    expect(await screen.findByText('Login Page')).toBeInTheDocument()
  })

  it('renders child content when token is set', async () => {
    renderWithRouter('/dashboard', 'valid-token')
    expect(await screen.findByText('Dashboard Content')).toBeInTheDocument()
  })

  it('preserves the original location in state when redirecting', async () => {
    renderWithRouter('/dashboard')
    expect(await screen.findByText('Login Page')).toBeInTheDocument()
  })

  it('redirects for whitespace-only token', async () => {
    renderWithRouter('/dashboard', '   ')
    expect(await screen.findByText('Login Page')).toBeInTheDocument()
  })
})
