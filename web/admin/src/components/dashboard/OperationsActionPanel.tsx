export function OperationsActionPanel() {
  return (
    <section className="operations-action-panel" aria-label="运维快捷动作">
      <header className="panel-header">
        <h2>[ QUICK ACTIONS ]</h2>
        <span>ops shortcuts</span>
      </header>
      <div className="operations-action-panel__grid">
        <button type="button">刷新全部</button>
        <button type="button">打开在线测试</button>
        <button type="button">导出错误日志</button>
        <button type="button">查看审计导出</button>
      </div>
    </section>
  )
}