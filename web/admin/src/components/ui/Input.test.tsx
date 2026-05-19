import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Input } from './Input'
import styles from './Input.module.css'

describe('Input', () => {
  it('renders with label', () => {
    render(<Input label="邮箱" />)
    expect(screen.getByLabelText('邮箱')).toBeInTheDocument()
  })

  it('shows error message', () => {
    render(<Input label="密码" error="密码不能为空" />)
    expect(screen.getByRole('alert')).toHaveTextContent('密码不能为空')
    expect(screen.getByLabelText('密码')).toHaveAttribute('aria-invalid', 'true')
  })

  it('applies aria-describedby for error', () => {
    render(<Input id="test" error="错误" />)
    const input = screen.getByRole('textbox')
    const error = screen.getByRole('alert')
    expect(input).toHaveAttribute('aria-describedby', error.id)
  })

  it('merges custom className', () => {
    const { container } = render(<Input className="custom" />)
    expect(container.querySelector('div')).toHaveClass('custom')
  })

  it('forwards ref', () => {
    const ref = { current: null }
    render(<Input ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLInputElement)
  })

  it('handles value change', async () => {
    const user = userEvent.setup()
    render(<Input label="用户名" />)
    const input = screen.getByLabelText('用户名')
    await user.type(input, 'test')
    expect(input).toHaveValue('test')
  })

  it('respects disabled prop', () => {
    render(<Input label="字段" disabled />)
    expect(screen.getByLabelText('字段')).toBeDisabled()
  })
})
