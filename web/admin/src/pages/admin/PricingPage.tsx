import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../../components/layout/AppShell'
import { listPricing, upsertPricing } from '../../lib/api/billing'
import type { PricingEntry } from '../../types/billing'

export function PricingPage() {
  const queryClient = useQueryClient()
  const [provider, setProvider] = useState('')
  const [model, setModel] = useState('')
  const [inputPrice, setInputPrice] = useState('')
  const [outputPrice, setOutputPrice] = useState('')
  const [isDefault, setIsDefault] = useState(false)
  const [editError, setEditError] = useState('')

  const pricingQuery = useQuery<{ object: string; data: PricingEntry[] }>({
    queryKey: ['admin-pricing'],
    queryFn: listPricing,
    refetchInterval: 30_000,
  })

  const upsertMutation = useMutation({
    mutationFn: upsertPricing,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-pricing'] })
      setProvider('')
      setModel('')
      setInputPrice('')
      setOutputPrice('')
      setIsDefault(false)
      setEditError('')
    },
    onError: (err: Error) => setEditError(err.message),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!provider.trim() || !inputPrice || !outputPrice) {
      setEditError('provider, input price, and output price are required')
      return
    }
    upsertMutation.mutate({
      provider: provider.trim(),
      model: isDefault ? '' : model.trim(),
      input_price_per_1k: parseFloat(inputPrice),
      output_price_per_1k: parseFloat(outputPrice),
      is_default: isDefault,
    })
  }

  return (
    <AppShell>
      <div className="page-header">
        <h2>定价管理</h2>
      </div>

      <div className="page-surface" style={{ marginBottom: '1rem' }}>
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>添加 / 更新定价</h3>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', maxWidth: 400 }}>
          <div>
            <label>Provider *</label>
            <input value={provider} onChange={e => setProvider(e.target.value)} placeholder="openai" />
          </div>
          <div>
            <label>Model（默认留空）</label>
            <input value={model} onChange={e => setModel(e.target.value)} placeholder="gpt-4" />
          </div>
          <div>
            <label>
              <input type="checkbox" checked={isDefault} onChange={e => setIsDefault(e.target.checked)} />
              {' '}设为 Provider 默认价
            </label>
          </div>
          <div>
            <label>输入价格（每 1K tokens）*</label>
            <input type="number" step="0.000001" value={inputPrice} onChange={e => setInputPrice(e.target.value)} />
          </div>
          <div>
            <label>输出价格（每 1K tokens）*</label>
            <input type="number" step="0.000001" value={outputPrice} onChange={e => setOutputPrice(e.target.value)} />
          </div>
          {editError && <div className="config-error" role="alert">{editError}</div>}
          <button type="submit" className="button-primary" disabled={upsertMutation.isPending}>
            {upsertMutation.isPending ? '保存中…' : '保存'}
          </button>
        </form>
      </div>

      <div className="page-surface">
        <h3 style={{ marginBottom: '0.75rem', fontWeight: 600 }}>当前定价</h3>
        {pricingQuery.isLoading && <div className="event-state">加载中…</div>}
        {pricingQuery.error && <div className="config-error" role="alert">加载失败</div>}
        {pricingQuery.data && (
          <table className="data-table">
            <thead>
              <tr>
                <th>Provider</th>
                <th>Model</th>
                <th>输入 $/1K</th>
                <th>输出 $/1K</th>
                <th>类型</th>
              </tr>
            </thead>
            <tbody>
              {pricingQuery.data.data.map((p: PricingEntry, i: number) => (
                <tr key={`${p.provider}-${p.model}-${i}`}>
                  <td>{p.provider}</td>
                  <td>{p.model || '—'}</td>
                  <td>{p.input_price_per_1k}</td>
                  <td>{p.output_price_per_1k}</td>
                  <td>{p.is_default ? '默认' : '模型覆盖'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </AppShell>
  )
}
