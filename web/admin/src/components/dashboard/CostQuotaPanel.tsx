export function CostQuotaPanel(props: {
  todayCost: string
  monthCost: string
  cacheHitRate: string
  providerErrorRate: string
  totalTokens: string
}) {
  return (
    <section className="cost-quota-panel" aria-label="成本与配额控制区">
      <header className="panel-header">
        <h2>成本与配额</h2>
        <span>预算监控</span>
      </header>
      <div className="cost-quota-panel__grid">
        <article>
          <span>今日成本</span>
          <strong>{props.todayCost}</strong>
        </article>
        <article>
          <span>本月成本</span>
          <strong>{props.monthCost}</strong>
        </article>
        <article>
          <span>缓存命中</span>
          <strong>{props.cacheHitRate}</strong>
        </article>
        <article>
          <span>错误率</span>
          <strong>{props.providerErrorRate}</strong>
        </article>
        <article>
          <span>总 Tokens</span>
          <strong>{props.totalTokens}</strong>
        </article>
      </div>
    </section>
  )
}
