import type { ChannelNode } from './dashboard-home.types'

export function ChannelHealthMatrix({
  channels,
  onSelect,
}: {
  channels: ChannelNode[]
  onSelect: (node: ChannelNode) => void
}) {
  return (
    <section className="channel-health-matrix" aria-label="渠道健康矩阵">
      <header className="panel-header">
        <h2>渠道健康</h2>
        <span>{channels.length} 个渠道</span>
      </header>
      {channels.length === 0 ? (
        <div className="channel-health-matrix__empty">暂无渠道数据</div>
      ) : (
        <div className="channel-health-matrix__grid">
          {channels.map((node) => (
            <button
              key={node.id}
              type="button"
              className={`channel-health-node ${node.status}`}
              onClick={() => onSelect(node)}
              aria-label={`渠道 ${node.name}，状态 ${node.status}，提供商 ${node.provider}`}
            >
              <div className="channel-health-node__header">
                <span className={`channel-health-node__dot channel-health-node__dot--${node.status}`} role="status" aria-label={node.status} />
                <strong>{node.name}</strong>
              </div>
              <div className="channel-health-node__meta">{node.provider}</div>
              <div className="channel-health-node__stats">
                <span>延迟 {node.latency}</span>
                <span>错误率 {node.errorRate}</span>
                <span>权重 {node.weight}</span>
                <span>请求 {node.requestCount}</span>
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}
