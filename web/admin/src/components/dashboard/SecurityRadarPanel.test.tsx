import { render, screen } from '@testing-library/react'
import { SecurityRadarPanel } from './SecurityRadarPanel'

it('renders empty-state security radar', () => {
  render(<SecurityRadarPanel events={[]} onSelectEvent={() => {}} />)
  expect(screen.getByText('[ SECURITY EVENTS ]')).toBeInTheDocument()
  expect(screen.getByText(/当前没有高风险事件/i)).toBeInTheDocument()
})

it('renders security events', () => {
  render(
    <SecurityRadarPanel
      events={[
        {
          id: 'evt-1',
          level: 'critical',
          title: 'Key brute-force suspected',
          summary: 'IP 1.2.3.4 attempted 500+ keys in 5 minutes',
          timestamp: '14:23:11',
          source: '1.2.3.4',
        },
      ]}
      onSelectEvent={() => {}}
    />
  )
  expect(screen.getByText('Key brute-force suspected')).toBeInTheDocument()
})