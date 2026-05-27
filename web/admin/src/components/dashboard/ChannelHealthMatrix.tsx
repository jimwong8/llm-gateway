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
        <h2>[ CHANNEL HEALTH ]</h2>
        <span>{channels.length} channels</span>
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
            >
              <div className="channel-health-node__header">
                <span className={`channel-health-node__dot channel-health-node__dot--${node.status}`} />
                <strong>{node.name}</strong>
              </div>
              <div className="channel-health-node__meta">{node.provider}</div>
              <div className="channel-health-node__stats">
                <span>{node.latency}</span>
                <span>{node.errorRate}</span>
                <span>w:{node.weight}</span>
                <span>req:{node.requestCount}</span>
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}