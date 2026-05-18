import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { SummaryMetricCard } from '../components/dashboard/SummaryMetricCard'
import { getBalance, getLedger } from '../lib/api/billing'

export function BillingPage() {
  const balanceQuery = useQuery({
    queryKey: ['billing-balance'],
    queryFn: getBalance,
    refetchInterval: 15_000,
  })

  const ledgerQuery = useQuery({
    queryKey: ['billing-ledger'],
    queryFn: () => getLedger({ limit: 50 }),
    refetchInterval: 30_000,
  })

  return (
    <AppShell>
      <div className="page-header">
        <h2>计费面板</h2>
      </div>

      <div className="summary-card-grid">
        <SummaryMetricCard
          label="钱包余额"
          value={balanceQuery.data ? `$${balanceQuery.data.balance.toFixed(4)}` : '—'}
        />
      </div>

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>交易流水</h3>
        {ledgerQuery.isLoading && <div className="event-state">加载中…</div>}
        {ledgerQuery.error && <div className="config-error" role="alert">加载失败</div>}
        {ledgerQuery.data && (
          <table className="data-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>类型</th>
                <th>金额</th>
                <th>描述</th>
              </tr>
            </thead>
            <tbody>
              {ledgerQuery.data.data.map(e => (
                <tr key={e.id}>
                  <td>{new Date(e.created_at).toLocaleString()}</td>
                  <td>{e.type}</td>
                  <td style={{ color: e.amount < 0 ? '#e53e3e' : '#38a169' }}>
                    {e.amount > 0 ? '+' : ''}{e.amount.toFixed(4)}
                  </td>
                  <td>{e.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </AppShell>
  )
}
