import { render, screen } from '@testing-library/react'
import { ChannelHealthMatrix } from './ChannelHealthMatrix'

it('renders channel nodes and status', () => {
  render(
    <ChannelHealthMatrix
      channels={[
        {
          id: 'ch-1',
          name: 'OPENAI-A',
          provider: 'openai',
          status: 'healthy',
          latency: '312ms',
          errorRate: '0.4%',
          weight: 10,
          requestCount: 1200,
        },
      ]}
      onSelect={() => {}}
    />
  )

  expect(screen.getByText('OPENAI-A')).toBeInTheDocument()
  expect(screen.getByText('openai')).toBeInTheDocument()
  expect(screen.getByText('312ms')).toBeInTheDocument()
})

it('renders empty state when no channels', () => {
  render(
    <ChannelHealthMatrix channels={[]} onSelect={() => {}} />
  )
  expect(screen.getByText(/暂无渠道数据/i)).toBeInTheDocument()
})