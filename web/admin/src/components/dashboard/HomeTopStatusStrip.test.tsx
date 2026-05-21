import { render, screen } from '@testing-library/react'
import { HomeTopStatusStrip } from './HomeTopStatusStrip'

it('renders top metrics in order', () => {
  render(
    <HomeTopStatusStrip
      metrics={[
        { id: 'rps', label: 'RPS', value: '1284', level: 'healthy' },
        { id: 'ttff', label: 'TTFF', value: '842ms', level: 'degraded' },
      ]}
      onClickMetric={() => {}}
    />
  )

  expect(screen.getByText('RPS')).toBeInTheDocument()
  expect(screen.getByText('1284')).toBeInTheDocument()
  expect(screen.getByText('TTFF')).toBeInTheDocument()
})
