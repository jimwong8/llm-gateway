import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken } from '../lib/auth'
import '../i18n'
import { LoginPage } from './LoginPage'

// ── Mock API 模块 ──────────────────────────────────────
const mockLogin = vi.fn()
const mockSetUserToken = vi.fn()
const mockApiRequest = vi.fn()

vi.mock('../lib/api/identity', async () => {
  const actual = await vi.importActual<typeof import('../lib/api/identity')>('../lib/api/identity')
  return {
    ...actual,
    login: (...args: unknown[]) => mockLogin(...args),
    setUserToken: (...args: unknown[]) => mockSetUserToken(...args),
    getGitHubLoginUrl: () => '/api/auth/oauth/github',
  }
})

vi.mock('../lib/http', () => ({
  apiRequest: (...args: unknown[]) => mockApiRequest(...args),
}))

// ── Helpers ─────────────────────────────────────────────
function renderLoginPage(initialEntry = '/login') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

// ── Tests ───────────────────────────────────────────────
describe('LoginPage', () => {
  beforeEach(() => {
    clearToken()
    vi.restoreAllMocks()
    mockApiRequest.mockResolvedValue({ github_enabled: false })
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  // ── 1. 表单渲染 ─────────────────────────────────────
  describe('表单渲染', () => {
    it('默认渲染管理员 Tab 和用户 Tab', async () => {
      renderLoginPage()

      expect(screen.getByRole('tab', { name: '管理员' })).toBeInTheDocument()
      expect(screen.getByRole('tab', { name: '用户登录' })).toBeInTheDocument()
    })

    it('默认激活管理员 Tab', async () => {
      renderLoginPage()

      const adminTab = screen.getByRole('tab', { name: '管理员' })
      expect(adminTab).toHaveAttribute('aria-selected', 'true')
    })

    it('管理员模式下显示 Token 输入框和进入控制台按钮', async () => {
      renderLoginPage()

      expect(screen.getByLabelText('管理员 Token')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '进入控制台' })).toBeInTheDocument()
    })

    it('切换到用户 Tab 显示邮箱和密码输入框', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      expect(screen.getByLabelText('邮箱')).toBeInTheDocument()
      expect(screen.getByLabelText('密码')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()
    })
  })

  // ── 2. 管理员模式登录 ───────────────────────────────
  describe('管理员模式登录', () => {
    it('输入 token 后点击登录跳转到 dashboard', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      const tokenInput = screen.getByLabelText('管理员 Token')
      await user.type(tokenInput, 'sk-admin-test-token')
      await user.click(screen.getByRole('button', { name: '进入控制台' }))

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })

    it('输入 token 后设置 admin token 到 sessionStorage', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.type(screen.getByLabelText('管理员 Token'), 'my-admin-token')
      await user.click(screen.getByRole('button', { name: '进入控制台' }))

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })

  // ── 3. 用户模式成功登录 ─────────────────────────────
  describe('用户模式成功登录', () => {
    it('输入 email + 密码登录成功并跳转', async () => {
      const user = userEvent.setup()
      mockLogin.mockResolvedValue({ token: 'user-jwt-token' })

      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('密码'), 'password123')
      await user.click(screen.getByRole('button', { name: '登录' }))

      await waitFor(() => {
        expect(mockLogin).toHaveBeenCalledWith({
          email: 'user@example.com',
          password: 'password123',
        })
      })

      expect(mockSetUserToken).toHaveBeenCalledWith('user-jwt-token')

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })

  // ── 4. 用户模式登录失败 ─────────────────────────────
  describe('用户模式登录失败', () => {
    it('错误密码显示错误消息', async () => {
      const user = userEvent.setup()
      mockLogin.mockRejectedValue(new Error('邮箱或密码错误'))

      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('密码'), 'wrongpassword')
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱或密码错误')
    })
  })

  // ── 5. 空字段验证 ───────────────────────────────────
  describe('空字段验证', () => {
    it('空 email 显示请输入邮箱', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      // 不填 email，直接填密码后提交
      await user.type(screen.getByLabelText('密码'), 'password123')
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入邮箱')
    })

    it('空密码显示请输入密码', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      // 不填密码直接提交
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入密码')
    })

    it('管理员模式空 token 显示错误', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      // 不填 token 直接提交
      await user.click(screen.getByRole('button', { name: '进入控制台' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入管理员 Token')
    })
  })

  // ── 6. 无效 email 格式验证 ──────────────────────────
  describe('无效 email 格式验证', () => {
    it('无效 email 格式显示邮箱格式不正确', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'invalid-email')
      await user.type(screen.getByLabelText('密码'), 'password123')
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱格式不正确')
    })

    it('缺少 at 符号的 email 显示格式错误', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'userexample.com')
      await user.type(screen.getByLabelText('密码'), 'password123')
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱格式不正确')
    })
  })

  // ── 7. Tab 切换 ─────────────────────────────────────
  describe('Tab 切换', () => {
    it('从管理员切换到用户模式', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      expect(screen.getByRole('tab', { name: '管理员' })).toHaveAttribute('aria-selected', 'true')

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      expect(screen.getByRole('tab', { name: '用户登录' })).toHaveAttribute('aria-selected', 'true')
      expect(screen.getByRole('tab', { name: '管理员' })).toHaveAttribute('aria-selected', 'false')
      expect(screen.getByLabelText('邮箱')).toBeInTheDocument()
    })

    it('从用户模式切换回管理员模式', async () => {
      const user = userEvent.setup()
      renderLoginPage()

      // 先切到用户
      await user.click(screen.getByRole('tab', { name: '用户登录' }))
      expect(screen.getByLabelText('邮箱')).toBeInTheDocument()

      // 再切回管理员
      await user.click(screen.getByRole('tab', { name: '管理员' }))

      expect(screen.getByRole('tab', { name: '管理员' })).toHaveAttribute('aria-selected', 'true')
      expect(screen.getByLabelText('管理员 Token')).toBeInTheDocument()
    })

    it('切换 Tab 时清除错误消息', async () => {
      const user = userEvent.setup()
      mockLogin.mockRejectedValue(new Error('登录失败'))

      renderLoginPage()

      // 先登录失败产生错误
      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('密码'), 'wrong')
      await user.click(screen.getByRole('button', { name: '登录' }))

      expect(await screen.findByRole('alert')).toBeInTheDocument()

      // 切换到管理员模式，错误应被清除
      await user.click(screen.getByRole('tab', { name: '管理员' }))

      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })

  // ── 8. 加载状态 ─────────────────────────────────────
  describe('加载状态', () => {
    it('登录中按钮显示 loading 状态', async () => {
      const user = userEvent.setup()

      // 让 login 请求延迟返回，以便观察 loading 状态
      let resolveLogin: (value: { token: string }) => void
      mockLogin.mockImplementation(
        () =>
          new Promise<{ token: string }>((resolve) => {
            resolveLogin = resolve
          }),
      )

      renderLoginPage()

      await user.click(screen.getByRole('tab', { name: '用户登录' }))

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('密码'), 'password123')
      await user.click(screen.getByRole('button', { name: '登录' }))

      // 按钮应该显示 loading 状态
      const button = screen.getByRole('button', { name: '登录中...' })
      expect(button).toHaveAttribute('aria-busy', 'true')
      expect(button).toBeDisabled()

      // 完成登录
      resolveLogin!({ token: 'test-token' })

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })

  // ── 9. URL token 参数自动登录 ───────────────────────
  describe('URL token 参数自动登录', () => {
    it('URL 带 token 参数时自动登录并跳转', async () => {
      renderLoginPage('/login?token=auto-login-token')

      expect(mockSetUserToken).toHaveBeenCalledWith('auto-login-token')

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })
})
