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
        <h2>[ COST / QUOTA ]</h2>
        <span>budget watch</span>
      </header>
      <div className="cost-quota-panel__grid">
        <article><span>TODAY COST</span><strong>{props.todayCost}</strong></article>
        <article><span>MONTH COST</span><strong>{props.monthCost}</strong></article>
        <article><span>CACHE HIT</span><strong>{props.cacheHitRate}</strong></article>
        <article><span>ERR RATE</span><strong>{props.providerErrorRate}</strong></article>
        <article><span>TOTAL TOKENS</span><strong>{props.totalTokens}</strong></article>
      </div>
    </section>
  )
}