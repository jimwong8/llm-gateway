import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { SummaryMetricCard } from '../components/dashboard/SummaryMetricCard'
import { getBalance, getLedger } from '../lib/api/billing'

export function BillingPage() {
  const { t } = useTranslation()
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
    <AppShell title={t('billing.title')} description={t('billing.description')}>
      <div className="page-header">
        <h2>{t('billing.title')}</h2>
      </div>

      <div className="summary-card-grid">
        <SummaryMetricCard
          label={t('billing.balance')}
          value={balanceQuery.data ? `$${balanceQuery.data.balance.toFixed(4)}` : '—'}
        />
      </div>

      <div className="page-surface" style={{ marginTop: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontSize: '1rem', fontWeight: 600 }}>{t('billing.ledger')}</h3>
        {ledgerQuery.isLoading && <div className="event-state">{t('common.loading')}</div>}
        {ledgerQuery.error && <div className="config-error" role="alert">{t('billing.loadError')}</div>}
        {ledgerQuery.data && (
          <table className="data-table">
            <thead>
              <tr>
                 <th>{t('billing.colTime')}</th>
                 <th>{t('billing.colType')}</th>
                 <th>{t('billing.colAmount')}</th>
                 <th>{t('billing.colDescription')}</th>
              </tr>
            </thead>
            <tbody>
              {(ledgerQuery.data.data ?? []).map(e => (
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
