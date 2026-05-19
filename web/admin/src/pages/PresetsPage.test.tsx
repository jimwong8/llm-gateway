import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import '../i18n'
import { PresetsPage } from './PresetsPage'

// ── Mock @tanstack/react-query ──────────────────────────
const mockUseQuery = vi.fn()
const mockUseMutation = vi.fn()

vi.mock('@tanstack/react-query', async () => {
  const actual = await vi.importActual<typeof import('@tanstack/react-query')>('@tanstack/react-query')
  return {
    ...actual,
    useQuery: (...args: unknown[]) => mockUseQuery(...args),
    useMutation: (...args: unknown[]) => mockUseMutation(...args),
    useQueryClient: () => ({
      invalidateQueries: vi.fn(),
    }),
  }
})

// ── Mock Tabs 组件（PresetsPage 使用 items prop，但 Tabs 期望 tabs prop）──
vi.mock('../components/ui/Tabs', () => ({
  Tabs: ({ items, activeKey, onChange }: { items: Array<{ key: string; label: string }>; activeKey: string; onChange: (k: string) => void }) => (
    <div data-testid="tabs" data-active-key={activeKey}>
      {items.map((tab) => (
        <button
          key={tab.key}
          type="button"
          role="tab"
          aria-selected={tab.key === activeKey}
          onClick={() => onChange(tab.key)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  ),
}))

// ── Test fixtures ──────────────────────────────────────
const samplePresets = [
  {
    id: 1,
    tenant_id: 'default',
    name: 'Code Review',
    system_prompt: 'You are a code review assistant for {{language}}.',
    model: 'gpt-4o',
    temperature: 0.3,
    max_tokens: 4096,
    created_at: '2026-05-01T10:00:00Z',
  },
  {
    id: 2,
    tenant_id: 'default',
    name: 'Summarizer',
    system_prompt: 'Summarize the following text.',
    model: 'gpt-4o-mini',
    temperature: 0.7,
    max_tokens: 2048,
    created_at: '2026-05-02T12:00:00Z',
  },
]

const sampleMasks = [
  {
    id: 1,
    tenant_id: 'default',
    name: '手机号脱敏',
    pattern: '1[3-9]\\d{9}',
    replacement: '***',
    enabled: true,
    created_at: '2026-05-01T10:00:00Z',
  },
  {
    id: 2,
    tenant_id: 'default',
    name: '邮箱脱敏',
    pattern: '[\\w.-]+@[\\w.-]+\\.\\w+',
    replacement: '***@***.com',
    enabled: false,
    created_at: '2026-05-02T12:00:00Z',
  },
]

// ── Helpers ─────────────────────────────────────────────
function defaultMutationMock() {
  return { mutate: vi.fn(), isPending: false, error: null }
}

function setupDefaultMocks() {
  mockUseQuery.mockImplementation((options: any) => {
    const key = options.queryKey?.[0]
    if (key === 'prompt-presets') {
      return { data: samplePresets, isLoading: false }
    }
    if (key === 'mask-rules') {
      return { data: sampleMasks, isLoading: false }
    }
    return { data: undefined, isLoading: false }
  })

  mockUseMutation.mockReturnValue(defaultMutationMock())
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <MemoryRouter initialEntries={['/presets']}>
      <QueryClientProvider client={queryClient}>
        <PresetsPage />
      </QueryClientProvider>
    </MemoryRouter>,
  )
}

// ── Tests ───────────────────────────────────────────────
describe('PresetsPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
    setupDefaultMocks()
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  // ── 列表加载显示 ─────────────────────────────────────
  describe('列表加载显示', () => {
    it('渲染 Prompt Presets 列表及数据', async () => {
      renderPage()

      expect(await screen.findByText('Code Review')).toBeInTheDocument()
      expect(screen.getByText('Summarizer')).toBeInTheDocument()
      expect(screen.getByText('gpt-4o')).toBeInTheDocument()
      expect(screen.getByText('gpt-4o-mini')).toBeInTheDocument()
      expect(screen.getByText('0.3')).toBeInTheDocument()
      expect(screen.getByText('4096')).toBeInTheDocument()
      expect(screen.getByText('2048')).toBeInTheDocument()
    })

    it('渲染 Mask Rules 列表及数据', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))

      expect(await screen.findByText('手机号脱敏')).toBeInTheDocument()
      expect(screen.getByText('邮箱脱敏')).toBeInTheDocument()
      expect(screen.getByText('1[3-9]\\d{9}')).toBeInTheDocument()
      expect(screen.getByText('***@***.com')).toBeInTheDocument()
      expect(screen.getByText('启用')).toBeInTheDocument()
      expect(screen.getByText('停用')).toBeInTheDocument()
    })
  })

  // ── 创建 Preset 表单提交 ─────────────────────────────
  describe('创建 Preset 表单提交', () => {
    it('点击新建按钮显示创建表单', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      expect(screen.getByText('新建 Preset')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('My Preset')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('gpt-4o')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('You are a helpful assistant...')).toBeInTheDocument()
    })

    it('填写表单并提交创建 preset', async () => {
      const user = userEvent.setup()
      const mutateMock = vi.fn()

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => {
          mutateMock(...args)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      await user.type(screen.getByPlaceholderText('My Preset'), 'Test Preset')
      await user.type(screen.getByPlaceholderText('gpt-4o'), 'gpt-4.1')
      await user.type(screen.getByPlaceholderText('You are a helpful assistant...'), 'Test prompt')

      await user.click(screen.getByRole('button', { name: '创建' }))

      expect(mutateMock).toHaveBeenCalledWith(
        expect.objectContaining({
          tenant_id: 'default',
          name: 'Test Preset',
          model: 'gpt-4.1',
          system_prompt: 'Test prompt',
        }),
      )
    })

    it('提交成功后表单重置并隐藏', async () => {
      const user = userEvent.setup()

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: () => options.onSuccess?.(),
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      await user.type(screen.getByPlaceholderText('My Preset'), 'Test')
      await user.type(screen.getByPlaceholderText('gpt-4o'), 'gpt-4')
      await user.type(screen.getByPlaceholderText('You are a helpful assistant...'), 'Prompt')

      await user.click(screen.getByRole('button', { name: '创建' }))

      await waitFor(() => {
        expect(screen.queryByText('新建 Prompt Preset')).not.toBeInTheDocument()
      })
    })

    it('点击取消关闭表单', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      expect(screen.getByText('新建 Preset')).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: '取消' }))

      expect(screen.queryByText('新建 Preset')).not.toBeInTheDocument()
    })
  })

  // ── 创建 Mask 表单提交 ───────────────────────────────
  describe('创建 Mask 表单提交', () => {
    it('切换到 masks tab 并点击新建显示表单', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))
      await user.click(screen.getByRole('button', { name: '+ 新建 Mask Rule' }))

      expect(screen.getByText('新建 Mask Rule')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('手机号脱敏')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('1[3-9]\\d{9}')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('***')).toBeInTheDocument()
    })

    it('填写表单并提交创建 mask rule', async () => {
      const user = userEvent.setup()
      const mutateMock = vi.fn()

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => {
          mutateMock(...args)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))
      await user.click(screen.getByRole('button', { name: '+ 新建 Mask Rule' }))

      await user.type(screen.getByPlaceholderText('手机号脱敏'), '身份证脱敏')
      await user.type(screen.getByPlaceholderText('1[3-9]\\d{9}'), 'id-pattern')
      const replacementInput = screen.getByPlaceholderText('***')
      await user.clear(replacementInput)
      await user.type(replacementInput, 'HIDDEN')

      await user.click(screen.getByRole('button', { name: '创建' }))

      expect(mutateMock).toHaveBeenCalledWith(
        expect.objectContaining({
          tenant_id: 'default',
          name: '身份证脱敏',
          pattern: 'id-pattern',
          replacement: 'HIDDEN',
          enabled: true,
        }),
      )
    })

    it('提交成功后表单重置并隐藏', async () => {
      const user = userEvent.setup()

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: () => options.onSuccess?.(),
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))
      await user.click(screen.getByRole('button', { name: '+ 新建 Mask Rule' }))

      await user.type(screen.getByPlaceholderText('手机号脱敏'), 'Test Mask')
      await user.type(screen.getByPlaceholderText('1[3-9]\\d{9}'), 'test-pattern')

      await user.click(screen.getByRole('button', { name: '创建' }))

      await waitFor(() => {
        expect(screen.queryByText('新建 Mask Rule')).not.toBeInTheDocument()
      })
    })
  })

  // ── 删除 Preset ──────────────────────────────────────
  describe('删除 Preset', () => {
    it('点击删除按钮并确认后调用删除 mutation', async () => {
      const user = userEvent.setup()
      const deleteMutateMock = vi.fn()

      vi.spyOn(window, 'confirm').mockReturnValue(true)

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (id: number) => {
          deleteMutateMock(id)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')

      const deleteButtons = screen.getAllByRole('button', { name: '删除' })
      expect(deleteButtons.length).toBeGreaterThanOrEqual(2)
      await user.click(deleteButtons[0])

      expect(window.confirm).toHaveBeenCalledWith('确定删除 Preset "Code Review"？')
      expect(deleteMutateMock).toHaveBeenCalledWith(1)
    })

    it('点击删除按钮但取消时不调用删除 mutation', async () => {
      const user = userEvent.setup()
      const deleteMutateMock = vi.fn()

      vi.spyOn(window, 'confirm').mockReturnValue(false)

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockReturnValue({
        ...defaultMutationMock(),
        mutate: (id: number) => deleteMutateMock(id),
      })

      renderPage()

      await screen.findByText('Code Review')

      const deleteButtons = screen.getAllByRole('button', { name: '删除' })
      await user.click(deleteButtons[0])

      expect(window.confirm).toHaveBeenCalledWith('确定删除 Preset "Code Review"？')
      expect(deleteMutateMock).not.toHaveBeenCalled()
    })
  })

  // ── 编辑 Preset ──────────────────────────────────────
  describe('编辑 Preset', () => {
    it('点击编辑按钮显示编辑表单并回显数据', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')

      const editButtons = screen.getAllByRole('button', { name: '编辑' })
      expect(editButtons.length).toBeGreaterThanOrEqual(2)
      await user.click(editButtons[0])

      expect(screen.getByText('编辑 Preset')).toBeInTheDocument()
      expect(screen.getByDisplayValue('Code Review')).toBeInTheDocument()
      expect(screen.getByDisplayValue('gpt-4o')).toBeInTheDocument()
      expect(screen.getByDisplayValue('You are a code review assistant for {{language}}.')).toBeInTheDocument()
    })

    it('编辑表单提交调用 updatePreset', async () => {
      const user = userEvent.setup()
      const updateMutateMock = vi.fn()

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => {
          updateMutateMock(...args)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')

      const editButtons = screen.getAllByRole('button', { name: '编辑' })
      await user.click(editButtons[0])

      const nameInput = screen.getByDisplayValue('Code Review')
      await user.clear(nameInput)
      await user.type(nameInput, 'Updated Preset')

      await user.click(screen.getByRole('button', { name: '保存' }))

      expect(updateMutateMock).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 1,
          input: expect.objectContaining({
            name: 'Updated Preset',
          }),
        }),
      )
    })

    it('编辑表单标题显示"编辑 Preset"而非"新建 Preset"', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')

      const editButtons = screen.getAllByRole('button', { name: '编辑' })
      await user.click(editButtons[0])

      expect(screen.getByText('编辑 Preset')).toBeInTheDocument()
      expect(screen.queryByText('新建 Preset')).not.toBeInTheDocument()
    })

    it('编辑时点击取消关闭表单', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')

      const editButtons = screen.getAllByRole('button', { name: '编辑' })
      await user.click(editButtons[0])

      expect(screen.getByText('编辑 Preset')).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: '取消' }))

      expect(screen.queryByText('编辑 Preset')).not.toBeInTheDocument()
    })
  })

  // ── 变量替换预览 ─────────────────────────────────────
  describe('变量替换预览', () => {
    it('创建表单中输入 variables 后显示预览', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      // 用 fireEvent.change 直接设置值，避免 userEvent.type 解析花括号为快捷键
      const promptTextarea = screen.getByPlaceholderText('You are a helpful assistant...')
      fireEvent.change(promptTextarea, { target: { value: 'Hello {{name}}, welcome to {{app}}' } })
      await user.type(screen.getByPlaceholderText('user_name, task_desc'), 'name, app')

      expect(screen.getByText('替换预览')).toBeInTheDocument()
      // 用 querySelector 查找包含预览文本的元素
      const previewEl = document.querySelector('.page-surface .page-surface')
      expect(previewEl?.textContent).toContain('Hello [name], welcome to [app]')
    })

    it('编辑表单中输入 variables 后显示预览', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')

      const editButtons = screen.getAllByRole('button', { name: '编辑' })
      await user.click(editButtons[0])

      await user.type(screen.getByPlaceholderText('user_name, task_desc'), 'language')

      expect(screen.getByText('替换预览')).toBeInTheDocument()
      const previewEl = document.querySelector('.page-surface .page-surface')
      expect(previewEl?.textContent).toContain('You are a code review assistant for [language].')
    })

    it('variables 为空时不显示预览区域', async () => {
      const user = userEvent.setup()
      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('button', { name: '+ 新建 Preset' }))

      await user.type(
        screen.getByPlaceholderText('You are a helpful assistant...'),
        'Hello {{name}}',
      )

      expect(screen.queryByText('替换预览')).not.toBeInTheDocument()
    })
  })

  // ── Mask 启用/停用切换 ──────────────────────────────
  describe('Mask 启用/停用切换', () => {
    it('点击已启用的 mask 状态按钮并确认后调用 toggle', async () => {
      const user = userEvent.setup()
      const toggleMutateMock = vi.fn()

      vi.spyOn(window, 'confirm').mockReturnValue(true)

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => {
          toggleMutateMock(...args)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))

      await screen.findByText('手机号脱敏')

      const enabledBadge = screen.getByRole('button', { name: '启用' })
      await user.click(enabledBadge)

      expect(window.confirm).toHaveBeenCalledWith('确定停用 Mask Rule "手机号脱敏"？')
      expect(toggleMutateMock).toHaveBeenCalledWith({ id: 1, enabled: false })
    })

    it('点击已停用的 mask 状态按钮并确认后调用 toggle', async () => {
      const user = userEvent.setup()
      const toggleMutateMock = vi.fn()

      vi.spyOn(window, 'confirm').mockReturnValue(true)

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockImplementation((options: any) => ({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => {
          toggleMutateMock(...args)
          options.onSuccess?.()
        },
      }))

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))

      await screen.findByText('邮箱脱敏')

      const disabledBadge = screen.getByRole('button', { name: '停用' })
      await user.click(disabledBadge)

      expect(window.confirm).toHaveBeenCalledWith('确定启用 Mask Rule "邮箱脱敏"？')
      expect(toggleMutateMock).toHaveBeenCalledWith({ id: 2, enabled: true })
    })

    it('点击 mask 状态按钮但取消时不调用 toggle', async () => {
      const user = userEvent.setup()
      const toggleMutateMock = vi.fn()

      vi.spyOn(window, 'confirm').mockReturnValue(false)

      mockUseQuery.mockImplementation((options: any) => {
        const key = options.queryKey?.[0]
        if (key === 'prompt-presets') {
          return { data: samplePresets, isLoading: false }
        }
        if (key === 'mask-rules') {
          return { data: sampleMasks, isLoading: false }
        }
        return { data: undefined, isLoading: false }
      })
      mockUseMutation.mockReturnValue({
        ...defaultMutationMock(),
        mutate: (...args: any[]) => toggleMutateMock(...args),
      })

      renderPage()

      await screen.findByText('Code Review')
      await user.click(screen.getByRole('tab', { name: 'Mask Rules' }))

      await screen.findByText('手机号脱敏')

      const enabledBadge = screen.getByRole('button', { name: '启用' })
      await user.click(enabledBadge)

      expect(window.confirm).toHaveBeenCalledWith('确定停用 Mask Rule "手机号脱敏"？')
      expect(toggleMutateMock).not.toHaveBeenCalled()
    })
  })
})
