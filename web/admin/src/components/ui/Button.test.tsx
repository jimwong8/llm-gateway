import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from './Button'
import styles from './Button.module.css'

describe('Button', () => {
  it('renders children', () => {
    render(<Button>提交</Button>)
    expect(screen.getByRole('button', { name: '提交' })).toBeInTheDocument()
  })

  it('applies variant class', () => {
    const { container } = render(<Button variant="danger">删除</Button>)
    const btn = container.querySelector('button')
    expect(btn).toHaveClass(styles['variant-danger'])
  })

  it('applies size class', () => {
    const { container } = render(<Button size="sm">小</Button>)
    const btn = container.querySelector('button')
    expect(btn).toHaveClass(styles['size-sm'])
  })

  it('shows spinner when loading', () => {
    render(<Button loading>加载中</Button>)
    const btn = screen.getByRole('button')
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('aria-busy', 'true')
    expect(btn.querySelector('svg')).toBeInTheDocument()
  })

  it('merges custom className', () => {
    const { container } = render(<Button className="custom">按钮</Button>)
    expect(container.querySelector('button')).toHaveClass('custom')
  })

  it('forwards ref', () => {
    const ref = { current: null }
    render(<Button ref={ref}>ref</Button>)
    expect(ref.current).toBeInstanceOf(HTMLButtonElement)
  })

  it('triggers onClick', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<Button onClick={onClick}>点击</Button>)
    await user.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not trigger onClick when disabled', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>禁用</Button>)
    await user.click(screen.getByRole('button'))
    expect(onClick).not.toHaveBeenCalled()
  })
})
