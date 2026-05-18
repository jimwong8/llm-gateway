import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Dialog } from './Dialog'

describe('Dialog', () => {
  it('renders nothing when closed', () => {
    const { container } = render(
      <Dialog open={false} onClose={vi.fn()}>
        <p>内容</p>
      </Dialog>,
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders content when open', () => {
    render(
      <Dialog open onClose={vi.fn()} title="标题">
        <p>内容</p>
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toHaveTextContent('标题')
    expect(screen.getByText('内容')).toBeInTheDocument()
  })

  it('calls onClose when clicking backdrop', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<Dialog open onClose={onClose} title="标题"><p>内容</p></Dialog>)
    await user.click(screen.getByRole('presentation'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onClose on Escape', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<Dialog open onClose={onClose} title="标题"><p>内容</p></Dialog>)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not close when clicking inside dialog', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<Dialog open onClose={onClose} title="标题"><p>内容</p></Dialog>)
    await user.click(screen.getByRole('dialog'))
    expect(onClose).not.toHaveBeenCalled()
  })

  it('renders description', () => {
    render(<Dialog open onClose={vi.fn()} title="标题" description="描述"><p>内容</p></Dialog>)
    expect(screen.getByText('描述')).toBeInTheDocument()
  })

  it('has close button with aria-label', () => {
    render(<Dialog open onClose={vi.fn()} title="标题"><p>内容</p></Dialog>)
    expect(screen.getByRole('button', { name: '关闭' })).toBeInTheDocument()
  })

  it('merges custom className', () => {
    const { container } = render(
      <Dialog open onClose={vi.fn()} className="custom"><p>内容</p></Dialog>,
    )
    expect(container.querySelector('[role="dialog"]')).toHaveClass('custom')
  })
})
