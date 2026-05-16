import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { getRuntimeObserver } from '../lib/runtimeObserver'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  if (value.startsWith('0001-01-01')) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

export function RuntimeObserverPage() {
  const [draftEnvironment, setDraftEnvironment] = useState('prod')
  const [environment, setEnvironment] = useState('prod')

  const observerQuery = useQuery({
    queryKey: ['runtime-observer', environment],
    queryFn: () => getRuntimeObserver(environment, 20),
  })

  const runtimeFacts = useMemo(() => observerQuery.data?.facts.runtime_decisions ?? [], [observerQuery.data])
  const distributionFacts = useMemo(() => observerQuery.data?.facts.distribution_events ?? [], [observerQuery.data])
  const cacheEntries = useMemo(() => observerQuery.data?.cache.entries ?? [], [observerQuery.data])

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setEnvironment(draftEnvironment.trim() || 'prod')
  }

  return (
    <AppShell
      title="运行时观测"
      description="查看当前环境的活跃策略、解析器缓存与失效状态，以及最近分发/运行时事实。"
    >
      <div className="events-page">
        <form className="config-filters" aria-label="运行时观测筛选" onSubmit={handleSubmit}>
          <label>
            环境
            <input value={draftEnvironment} onChange={(event) => setDraftEnvironment(event.target.value)} placeholder="生产环境" />
          </label>
          <div className="config-filters__actions">
            <button type="submit">刷新观察数据</button>
          </div>
        </form>

        {observerQuery.isLoading ? <div className="event-state">正在加载 runtime observer 数据…</div> : null}
        {observerQuery.error ? <div className="config-error">runtime observer 加载失败，请检查 governance/runtime 服务状态。</div> : null}

        {!observerQuery.isLoading && !observerQuery.error && observerQuery.data ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>环境</span>
                <strong>{observerQuery.data.environment}</strong>
              </section>
              <section className="summary-card">
                <span>活跃策略</span>
                <strong>{observerQuery.data.active_policy.version_id || '—'}</strong>
                <small>{formatDate(observerQuery.data.active_policy.updated_at)}</small>
              </section>
              <section className="summary-card">
                <span>缓存条目</span>
                <strong>{observerQuery.data.cache.entry_count}</strong>
              </section>
              <section className="summary-card">
                <span>失效次数</span>
                <strong>{observerQuery.data.cache.invalidation_count}</strong>
                <small>{formatDate(observerQuery.data.cache.last_invalidated_at)}</small>
              </section>
            </div>

            <div className="event-table">
              <table>
                <thead>
                  <tr>
                    <th>缓存环境</th>
                    <th>策略版本</th>
                    <th>缓存时间</th>
                  </tr>
                </thead>
                <tbody>
                  {cacheEntries.map((entry) => (
                    <tr key={`${entry.environment}-${entry.policy_version_id}-${entry.cached_at}`}>
                      <td>{entry.environment}</td>
                      <td>{entry.policy_version_id}</td>
                      <td>{formatDate(entry.cached_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="runtime-observer-grid">
              <section className="event-table">
                <table>
                  <thead>
                    <tr>
                      <th>请求 ID</th>
                      <th>解析模型</th>
                      <th>作用域</th>
                      <th>创建时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runtimeFacts.map((fact) => (
                      <tr key={fact.request_id}>
                        <td>{fact.request_id}</td>
                        <td>{fact.resolved_model}</td>
                        <td>{fact.matched_scope_type || '—'}</td>
                        <td>{formatDate(fact.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>

              <section className="event-table">
                <table>
                  <thead>
                    <tr>
                      <th>事件 ID</th>
                      <th>类型</th>
                      <th>发布</th>
                      <th>创建时间</th>
                    </tr>
                  </thead>
                  <tbody>
                    {distributionFacts.map((fact) => (
                      <tr key={fact.event_id}>
                        <td>{fact.event_id}</td>
                        <td>{fact.event_type}</td>
                        <td>{fact.rollout_id || '—'}</td>
                        <td>{formatDate(fact.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            </div>
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
