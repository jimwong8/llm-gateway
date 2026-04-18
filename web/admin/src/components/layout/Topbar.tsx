export function Topbar({ onToggleNavigation }: { onToggleNavigation: () => void }) {
  return (
    <header className="topbar">
      <div className="topbar__left">
        <button type="button" aria-label="Toggle navigation" onClick={onToggleNavigation}>
          菜单
        </button>
        <div>
          <strong>LLM Gateway Console</strong>
          <p>同进程管理台与在线测试台</p>
        </div>
      </div>
      <div className="topbar__right">
        <span className="env-badge">Environment: Local</span>
      </div>
    </header>
  )
}
