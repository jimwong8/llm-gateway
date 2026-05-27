import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import '../i18n'
import { SignupPage } from './SignupPage'

// ── Mock API 模块 ──────────────────────────────────────
const mockSignup = vi.fn()
const mockSetUserToken = vi.fn()

vi.mock('../lib/api/identity', async () => {
  const actual = await vi.importActual<typeof import('../lib/api/identity')>('../lib/api/identity')
  return {
    ...actual,
    signup: (...args: unknown[]) => mockSignup(...args),
    setUserToken: (...args: unknown[]) => mockSetUserToken(...args),
  }
})

// ── Helpers ─────────────────────────────────────────────
function renderSignupPage() {
  return render(
    <MemoryRouter initialEntries={['/signup']}>
      <Routes>
        <Route path="/signup" element={<SignupPage />} />
        <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

// ── Tests ───────────────────────────────────────────────
describe('SignupPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // ── 1. 表单渲染 ─────────────────────────────────────
  describe('表单渲染', () => {
    it('渲染所有表单字段', () => {
      renderSignupPage()

      expect(screen.getByLabelText('邮箱')).toBeInTheDocument()
      expect(screen.getByLabelText('用户名')).toBeInTheDocument()
      expect(screen.getByLabelText('密码')).toBeInTheDocument()
      expect(screen.getByLabelText('确认密码')).toBeInTheDocument()
    })

    it('渲染注册按钮', () => {
      renderSignupPage()

      expect(screen.getByRole('button', { name: '注册' })).toBeInTheDocument()
    })

    it('渲染返回登录链接', () => {
      renderSignupPage()

      expect(screen.getByText('已有账号？')).toBeInTheDocument()
      expect(screen.getByRole('link', { name: '登录' })).toBeInTheDocument()
    })
  })

  // ── 2. 成功注册 ─────────────────────────────────────
  describe('成功注册', () => {
    it('填写所有字段提交后跳转', async () => {
      const user = userEvent.setup()
      mockSignup.mockResolvedValue({ token: 'new-user-token' })

      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'newuser@example.com')
      await user.type(screen.getByLabelText('用户名'), 'newuser')
      await user.type(screen.getByLabelText('密码'), 'StrongP@ss1')
      await user.type(screen.getByLabelText('确认密码'), 'StrongP@ss1')

      await user.click(screen.getByRole('button', { name: '注册' }))

      await waitFor(() => {
        expect(mockSignup).toHaveBeenCalledWith({
          email: 'newuser@example.com',
          username: 'newuser',
          password: 'StrongP@ss1',
        })
      })

      expect(mockSetUserToken).toHaveBeenCalledWith('new-user-token')

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })

  // ── 3. 密码不匹配 ───────────────────────────────────
  describe('密码不匹配', () => {
    it('密码不一致时显示错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'testuser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('密码'), 'DifferentPass!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('两次输入的密码不一致')
    })

    it('确认密码与密码不一致时显示行内错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'DifferentPass!')

      expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument()
    })
  })

  // ── 4. 弱密码提示 ───────────────────────────────────
  describe('弱密码提示', () => {
    it('纯数字密码显示弱密码提示', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('密码'), '12345678')

      expect(screen.getByText('弱')).toBeInTheDocument()
    })

    it('8位以上混合大小写字母和数字显示中等强度', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('密码'), 'Password1')

      expect(screen.getByText('中')).toBeInTheDocument()
    })

    it('12位以上含特殊字符显示强密码', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('密码'), 'StrongP@ssword1')

      expect(screen.getByText('强')).toBeInTheDocument()
    })

    it('密码少于 8 位不显示强度提示', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('密码'), 'short')

      expect(screen.queryByText('弱')).not.toBeInTheDocument()
      expect(screen.queryByText('中')).not.toBeInTheDocument()
      expect(screen.queryByText('强')).not.toBeInTheDocument()
    })
  })

  // ── 5. 无效 email 验证 ──────────────────────────────
  describe('无效 email 验证', () => {
    it('无效 email 格式显示错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'invalid-email')
      await user.type(screen.getByLabelText('用户名'), 'testuser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱格式不正确')
    })

    it('缺少 at 符号的 email 显示格式错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'userexample.com')
      await user.type(screen.getByLabelText('用户名'), 'testuser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱格式不正确')
    })
  })

  // ── 6. 空字段验证 ───────────────────────────────────
  describe('空字段验证', () => {
    it('空 email 显示请输入邮箱', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('用户名'), 'testuser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入邮箱')
    })

    it('空用户名显示请输入用户名', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入用户名')
    })

    it('空密码显示请输入密码', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'testuser')
      // 不填密码和确认密码，避免行内密码不匹配错误干扰
      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('请输入密码')
    })
  })

  // ── 7. 用户名验证 ───────────────────────────────────
  describe('用户名验证', () => {
    it('用户名太短显示错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'a')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent(
        '用户名需 2-32 个字符，支持中英文、数字、下划线和连字符',
      )
    })

    it('用户名太长显示错误', async () => {
      const user = userEvent.setup()
      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'a'.repeat(33))
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent(
        '用户名需 2-32 个字符，支持中英文、数字、下划线和连字符',
      )
    })

    it('有效用户名通过验证', async () => {
      const user = userEvent.setup()
      mockSignup.mockResolvedValue({ token: 'token' })

      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'valid_user-123')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      await waitFor(() => {
        expect(mockSignup).toHaveBeenCalled()
      })
    })

    it('中文用户名通过验证', async () => {
      const user = userEvent.setup()
      mockSignup.mockResolvedValue({ token: 'token' })

      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), '张三')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      await waitFor(() => {
        expect(mockSignup).toHaveBeenCalled()
      })
    })

    it('注册失败显示错误消息', async () => {
      const user = userEvent.setup()
      mockSignup.mockRejectedValue(new Error('邮箱已被注册'))

      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'existing@example.com')
      await user.type(screen.getByLabelText('用户名'), 'existinguser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      expect(await screen.findByRole('alert')).toHaveTextContent('邮箱已被注册')
    })

    it('注册中显示 loading 状态', async () => {
      const user = userEvent.setup()

      let resolveSignup: (value: { token: string }) => void
      mockSignup.mockImplementation(
        () =>
          new Promise<{ token: string }>((resolve) => {
            resolveSignup = resolve
          }),
      )

      renderSignupPage()

      await user.type(screen.getByLabelText('邮箱'), 'user@example.com')
      await user.type(screen.getByLabelText('用户名'), 'testuser')
      await user.type(screen.getByLabelText('密码'), 'Password123!')
      await user.type(screen.getByLabelText('确认密码'), 'Password123!')

      await user.click(screen.getByRole('button', { name: '注册' }))

      const button = screen.getByRole('button', { name: '注册中...' })
      expect(button).toHaveAttribute('aria-busy', 'true')
      expect(button).toBeDisabled()

      resolveSignup!({ token: 'test-token' })

      await waitFor(() => {
        expect(screen.getByTestId('dashboard')).toBeInTheDocument()
      })
    })
  })
})
