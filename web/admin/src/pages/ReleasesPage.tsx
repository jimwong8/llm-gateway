import { useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { PromotionPanel } from '../components/releases/PromotionPanel'
import { ReleaseDraftPanel } from '../components/releases/ReleaseDraftPanel'
import type { ConfigVersion } from '../types/admin'

export function ReleasesPage() {
  const [lastResult, setLastResult] = useState<ConfigVersion | null>(null)

  return (
    <AppShell
      title="Releases"
      description="在一个工作台里完成 Draft 发布与跨环境 Promotion，并查看最近一次操作回执。"
    >
      <div className="releases-page">
        <div className="releases-grid">
          <ReleaseDraftPanel onReleased={setLastResult} />
          <PromotionPanel onPromoted={setLastResult} />
        </div>

        <section className="release-result-card">
          <h2>最近一次结果</h2>
          {lastResult ? (
            <dl className="release-result-grid">
              <div>
                <dt>Version ID</dt>
                <dd>{lastResult.version_id}</dd>
              </div>
              <div>
                <dt>Status</dt>
                <dd>{lastResult.status}</dd>
              </div>
              <div>
                <dt>Environment</dt>
                <dd>{lastResult.environment}</dd>
              </div>
              <div>
                <dt>Source</dt>
                <dd>
                  {lastResult.source
                    ? `${lastResult.source.source_environment} / ${lastResult.source.source_version_id}`
                    : '—'}
                </dd>
              </div>
            </dl>
          ) : (
            <div className="config-drawer__empty">完成一次 release 或 promotion 后，这里会显示最新结果。</div>
          )}
        </section>
      </div>
    </AppShell>
  )
}
