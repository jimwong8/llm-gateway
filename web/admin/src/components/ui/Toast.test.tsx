import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Toast, ToastProvider, useToast } from './Toast'

describe('Toast', () => {
  it('renders message', () => {
    render(<Toast message="操作成功" />)
    expect(screen.getByRole('alert')).toHaveTextContent('操作成功')
  })

  it('calls onDismiss when close button clicked', async () => {
    const user = userEvent.setup()
    const onDismiss = vi.fn()
    render(<Toast message="提示" onDismiss={onDismiss} />)
    await user.click(screen.getByRole('button', { name: '关闭通知' }))
    expect(onDismiss).toHaveBeenCalledOnce()
  })
})

describe('ToastProvider', () => {
  function TestComponent() {
    const { addToast } = useToast()
    return <button onClick={() => addToast('test', 'success')}>触发</button>
  }

  it('provides toast via context', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <TestComponent />
      </ToastProvider>,
    )
    await user.click(screen.getByText('触发'))
    expect(screen.getByRole('alert')).toHaveTextContent('test')
  })

  it('throws outside provider', () => {
    expect(() => render(<TestComponent />)).toThrow('useToast must be used within a ToastProvider')
  })
})
