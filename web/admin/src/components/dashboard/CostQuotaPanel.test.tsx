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
    />,
  )

  expect(screen.getByText('成本与配额')).toBeInTheDocument()
  expect(screen.getByText('预算监控')).toBeInTheDocument()
  expect(screen.getByText('今日成本')).toBeInTheDocument()
  expect(screen.getByText('本月成本')).toBeInTheDocument()
  expect(screen.getByText('缓存命中')).toBeInTheDocument()
  expect(screen.getByText('错误率')).toBeInTheDocument()
  expect(screen.getByText('总 Tokens')).toBeInTheDocument()
  expect(screen.getByText('$184.23')).toBeInTheDocument()
  expect(screen.getByText('$3940.20')).toBeInTheDocument()
  expect(screen.getByText('24.5%')).toBeInTheDocument()
  expect(screen.getByText('1.2%')).toBeInTheDocument()
  expect(screen.getByText('1284000')).toBeInTheDocument()
})

it('renders placeholder dashes when cost data missing', () => {
  render(
    <CostQuotaPanel
      todayCost="--"
      monthCost="--"
      cacheHitRate="0.0%"
      providerErrorRate="0.0%"
      totalTokens="0"
    />,
  )
  expect(screen.getAllByText('--').length).toBeGreaterThanOrEqual(2)
})
