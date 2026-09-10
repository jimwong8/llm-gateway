import type { ConfigVersion } from '../../types/admin'

type ConfigVersionTableProps = {
  versions: ConfigVersion[]
  loading: boolean
  onSelect: (version: ConfigVersion) => void
}

export function ConfigVersionTable({ versions, loading, onSelect }: ConfigVersionTableProps) {
  if (loading) {
    return <div className="config-table__state">正在加载配置版本…</div>
  }

  if (versions.length === 0) {
    return <div className="config-table__state">当前筛选条件下没有配置版本。</div>
  }

  return (
    <div className="config-table">
      <table>
        <thead>
          <tr>
            <th>版本 ID</th>
            <th>状态</th>
            <th>环境</th>
            <th>来源</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {versions.map((version) => (
            <tr key={version.version_id}>
              <td>{version.version_id}</td>
              <td>
                <span className={`status-pill ${version.status}`}>{version.status}</span>
              </td>
              <td>{version.environment}</td>
              <td>
                {version.source
                  ? `${version.source.source_environment} / ${version.source.source_version_id}`
                  : '—'}
              </td>
              <td>
                <button type="button" onClick={() => onSelect(version)}>
                  查看详情 {version.version_id}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
