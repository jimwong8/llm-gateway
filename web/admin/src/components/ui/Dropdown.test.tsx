import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Dropdown } from './Dropdown'

const items = [
  { key: 'edit', label: '编辑', onClick: vi.fn() },
  { key: 'delete', label: '删除', onClick: vi.fn(), danger: true },
]

describe('Dropdown', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders trigger', () => {
    render(<Dropdown trigger={<span>菜单</span>} items={items} />)
    expect(screen.getByText('菜单')).toBeInTheDocument()
  })

  it('opens menu on trigger click', async () => {
    const user = userEvent.setup()
    render(<Dropdown trigger={<span>菜单</span>} items={items} />)
    await user.click(screen.getByText('菜单'))
    expect(screen.getByText('编辑')).toBeInTheDocument()
    expect(screen.getByText('删除')).toBeInTheDocument()
  })

  it('calls item onClick and closes menu', async () => {
    const user = userEvent.setup()
    render(<Dropdown trigger={<span>菜单</span>} items={items} />)
    await user.click(screen.getByText('菜单'))
    await user.click(screen.getByText('编辑'))
    expect(items[0].onClick).toHaveBeenCalledOnce()
    expect(screen.queryByText('编辑')).not.toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    const user = userEvent.setup()
    render(<Dropdown trigger={<span>菜单</span>} items={items} />)
    await user.click(screen.getByText('菜单'))
    await user.keyboard('{Escape}')
    expect(screen.queryByText('编辑')).not.toBeInTheDocument()
  })

  it('has proper ARIA attributes', async () => {
    const user = userEvent.setup()
    render(<Dropdown trigger={<span>菜单</span>} items={items} />)
    const trigger = screen.getByText('菜单').closest('button')!
    expect(trigger).toHaveAttribute('aria-haspopup', 'true')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  it('merges custom className', () => {
    const { container } = render(<Dropdown trigger={<span>菜单</span>} items={items} className="custom" />)
    expect(container.firstChild).toHaveClass('custom')
  })
})
