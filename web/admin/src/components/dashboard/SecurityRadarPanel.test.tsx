import { render, screen } from '@testing-library/react'
import { SecurityRadarPanel } from './SecurityRadarPanel'

it('renders empty-state security radar', () => {
  render(<SecurityRadarPanel events={[]} onSelectEvent={() => {}} />)
  expect(screen.getByText('安全事件')).toBeInTheDocument()
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
    />,
  )
  expect(screen.getByText('Key brute-force suspected')).toBeInTheDocument()
  expect(screen.getByText(/attempted 500\+ keys/)).toBeInTheDocument()
})

it('renders loading state', () => {
  render(<SecurityRadarPanel events={[]} onSelectEvent={() => {}} loading />)
  expect(screen.getByText(/正在加载安全事件/)).toBeInTheDocument()
})

it('renders error state with Chinese translation', () => {
  render(
    <SecurityRadarPanel
      events={[]}
      onSelectEvent={() => {}}
      error={new Error('channel rate gate: channel rate limit')}
    />,
  )
  expect(screen.getByText('渠道限流触发')).toBeInTheDocument()
})
