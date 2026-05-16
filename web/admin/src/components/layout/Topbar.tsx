export function Topbar({ onToggleNavigation }: { onToggleNavigation: () => void }) {
  return (
    <header className="topbar">
      <div className="topbar__left">
        <button type="button" aria-label="切换导航" onClick={onToggleNavigation}>
          菜单
        </button>
        <div>
          <strong>LLM Gateway Console</strong>
          <p>管理控制台与在线测试台</p>
        </div>
      </div>
      <div className="topbar__right">
        <span className="env-badge">环境: Local</span>
      </div>
    </header>
  )
}
