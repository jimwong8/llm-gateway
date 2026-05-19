import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Select } from './Select'

const options = [
  { value: 'a', label: '选项 A' },
  { value: 'b', label: '选项 B' },
]

describe('Select', () => {
  it('renders with label', () => {
    render(<Select label="类型" options={options} />)
    expect(screen.getByLabelText('类型')).toBeInTheDocument()
  })

  it('renders options', () => {
    render(<Select options={options} />)
    expect(screen.getByText('选项 A')).toBeInTheDocument()
    expect(screen.getByText('选项 B')).toBeInTheDocument()
  })

  it('shows placeholder', () => {
    render(<Select options={options} placeholder="请选择" />)
    expect(screen.getByText('请选择')).toBeInTheDocument()
  })

  it('shows error message', () => {
    render(<Select label="类型" options={options} error="请选择类型" />)
    expect(screen.getByRole('alert')).toHaveTextContent('请选择类型')
    expect(screen.getByLabelText('类型')).toHaveAttribute('aria-invalid', 'true')
  })

  it('handles value change', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<Select options={options} onChange={onChange} />)
    const select = screen.getByRole('combobox')
    await user.selectOptions(select, 'b')
    expect(onChange).toHaveBeenCalled()
  })

  it('merges custom className', () => {
    const { container } = render(<Select options={options} className="custom" />)
    expect(container.querySelector('div')).toHaveClass('custom')
  })

  it('forwards ref', () => {
    const ref = { current: null }
    render(<Select options={options} ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLSelectElement)
  })
})
