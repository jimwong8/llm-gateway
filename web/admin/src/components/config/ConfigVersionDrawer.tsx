import type { ConfigVersion } from '../../types/admin'

type ConfigVersionDrawerProps = {
  version: ConfigVersion | null
  onClose: () => void
}

export function ConfigVersionDrawer({ version, onClose }: ConfigVersionDrawerProps) {
  return (
    <aside className="config-drawer" aria-label="版本详情" data-open={String(Boolean(version))}>
      <div className="config-drawer__header">
        <div>
          <h2>版本详情</h2>
          <p>查看配置版本的发布状态与继承来源。</p>
        </div>
        <button type="button" onClick={onClose} disabled={!version}>
          关闭
        </button>
      </div>

      {version ? (
        <dl className="config-drawer__grid">
          <div>
            <dt>版本 ID</dt>
            <dd>{version.version_id}</dd>
          </div>
          <div>
            <dt>状态</dt>
            <dd>{version.status}</dd>
          </div>
          <div>
            <dt>环境</dt>
            <dd>{version.environment}</dd>
          </div>
          <div>
            <dt>来源类型</dt>
            <dd>{version.source?.type ?? '—'}</dd>
          </div>
          <div>
            <dt>来源环境</dt>
            <dd>{version.source?.source_environment ?? '—'}</dd>
          </div>
          <div>
            <dt>来源版本</dt>
            <dd>{version.source?.source_version_id ?? '—'}</dd>
          </div>
        </dl>
      ) : (
        <div className="config-drawer__empty">请选择左侧任一版本查看详情。</div>
      )}
    </aside>
  )
}
