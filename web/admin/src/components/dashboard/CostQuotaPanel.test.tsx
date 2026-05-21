import { render, screen } from '@testing-library/react'
import { CostQuotaPanel } from './CostQuotaPanel'

it('renders cost and quota summary', () => {
  render(
    <CostQuotaPanel
      todayCost="$184.23"
      monthCost="$3940.20"
      cacheHitRate="24.5%"
      providerErrorRate="1.2%"
      totalTokens="1284000"
    />
  )

  expect(screen.getByText('$184.23')).toBeInTheDocument()
  expect(screen.getByText('$3940.20')).toBeInTheDocument()
  expect(screen.getByText('24.5%')).toBeInTheDocument()
})