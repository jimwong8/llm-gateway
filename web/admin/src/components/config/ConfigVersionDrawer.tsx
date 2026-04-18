import type { ConfigVersion } from '../../types/admin'

type ConfigVersionDrawerProps = {
  version: ConfigVersion | null
  onClose: () => void
}

export function ConfigVersionDrawer({ version, onClose }: ConfigVersionDrawerProps) {
  return (
    <aside className="config-drawer" aria-label="Version details" data-open={String(Boolean(version))}>
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
            <dt>Version ID</dt>
            <dd>{version.version_id}</dd>
          </div>
          <div>
            <dt>Status</dt>
            <dd>{version.status}</dd>
          </div>
          <div>
            <dt>Environment</dt>
            <dd>{version.environment}</dd>
          </div>
          <div>
            <dt>Source Type</dt>
            <dd>{version.source?.type ?? '—'}</dd>
          </div>
          <div>
            <dt>Source Environment</dt>
            <dd>{version.source?.source_environment ?? '—'}</dd>
          </div>
          <div>
            <dt>Source Version</dt>
            <dd>{version.source?.source_version_id ?? '—'}</dd>
          </div>
        </dl>
      ) : (
        <div className="config-drawer__empty">请选择左侧任一版本查看详情。</div>
      )}
    </aside>
  )
}
