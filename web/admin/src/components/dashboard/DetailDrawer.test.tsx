import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DetailDrawer } from './DetailDrawer'

it('opens and closes drawer', async () => {
  const onClose = vi.fn()
  render(
    <DetailDrawer open title="OPENAI-A" onClose={onClose}>
      <div>detail body</div>
    </DetailDrawer>
  )

  expect(screen.getByText('OPENAI-A')).toBeInTheDocument()
  expect(screen.getByText('detail body')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: /close/i }))
  expect(onClose).toHaveBeenCalled()
})

it('renders nothing when closed', () => {
  const { container } = render(
    <DetailDrawer open={false} title="Test" onClose={() => {}}>
      <div>body</div>
    </DetailDrawer>
  )
  expect(container.firstChild).toBeNull()
})
