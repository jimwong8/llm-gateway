import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Tabs } from './Tabs'

const tabs = [
  { key: 'a', label: '标签 A' },
  { key: 'b', label: '标签 B' },
]

describe('Tabs', () => {
  it('renders all tabs', () => {
    render(<Tabs tabs={tabs} activeKey="a" onChange={vi.fn()} />)
    expect(screen.getByText('标签 A')).toBeInTheDocument()
    expect(screen.getByText('标签 B')).toBeInTheDocument()
  })

  it('highlights active tab', () => {
    render(<Tabs tabs={tabs} activeKey="a" onChange={vi.fn()} />)
    expect(screen.getByRole('tab', { name: '标签 A' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: '标签 B' })).toHaveAttribute('aria-selected', 'false')
  })

  it('calls onChange on click', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Tabs tabs={tabs} activeKey="a" onChange={onChange} />)
    await user.click(screen.getByText('标签 B'))
    expect(onChange).toHaveBeenCalledWith('b')
  })

  it('has proper role structure', () => {
    render(<Tabs tabs={tabs} activeKey="a" onChange={vi.fn()} />)
    expect(screen.getByRole('tablist')).toBeInTheDocument()
  })

  it('renders children as tabpanel', () => {
    render(
      <Tabs tabs={tabs} activeKey="a" onChange={vi.fn()}>
        <p>面板内容</p>
      </Tabs>,
    )
    expect(screen.getByRole('tabpanel')).toHaveTextContent('面板内容')
  })

  it('merges custom className', () => {
    const { container } = render(<Tabs tabs={tabs} activeKey="a" onChange={vi.fn()} className="custom" />)
    expect(container.firstChild).toHaveClass('custom')
  })
})
