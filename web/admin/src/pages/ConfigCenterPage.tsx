import { FormEvent, useMemo, useState } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { CreateDraftForm } from '../components/config/CreateDraftForm'
import { ConfigVersionDrawer } from '../components/config/ConfigVersionDrawer'
import { ConfigVersionTable } from '../components/config/ConfigVersionTable'
import { useConfigVersions } from '../hooks/useConfigVersions'
import type { ConfigVersion, ConfigVersionFilters } from '../types/admin'

const emptyFilters: ConfigVersionFilters = {
  module: '',
  tenantID: '',
  environment: '',
  scope: '',
  projectID: '',
}

export function ConfigCenterPage() {
  const [draftFilters, setDraftFilters] = useState<ConfigVersionFilters>(emptyFilters)
  const [appliedFilters, setAppliedFilters] = useState<ConfigVersionFilters>(emptyFilters)
  const [selectedVersion, setSelectedVersion] = useState<ConfigVersion | null>(null)

  const query = useConfigVersions(appliedFilters)
  const versions = useMemo(() => query.data ?? [], [query.data])

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setAppliedFilters({ ...draftFilters })
  }

  return (
    <AppShell
      title="配置中心"
      description="查看配置版本列表、筛选结果，并在右侧详情抽屉里检查继承来源。"
    >
      <div className="config-center">
        <CreateDraftForm
          onCreated={(version) => {
            setSelectedVersion(version)
            void query.refetch()
          }}
        />

        <form className="config-filters" aria-label="配置筛选" onSubmit={handleSubmit}>
          <label>
            模块
            <input
              value={draftFilters.module}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, module: event.target.value }))}
              placeholder="路由模块"
            />
          </label>
          <label>
            租户 ID
            <input
              value={draftFilters.tenantID}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, tenantID: event.target.value }))}
              placeholder="租户-a"
            />
          </label>
          <label>
            环境
            <input
              value={draftFilters.environment}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, environment: event.target.value }))}
              placeholder="生产环境"
            />
          </label>
          <label>
            作用域
            <input
              value={draftFilters.scope}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, scope: event.target.value }))}
              placeholder="租户"
            />
          </label>
          <label>
            项目 ID
            <input
              value={draftFilters.projectID}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, projectID: event.target.value }))}
              placeholder="项目-x"
            />
          </label>
          <div className="config-filters__actions">
            <button type="submit">筛选</button>
          </div>
        </form>

        {query.error ? (
          <div className="config-error">加载配置版本失败，请检查 Admin Token 或接口状态。</div>
        ) : null}

        <div className="config-center__content">
          <ConfigVersionTable
            versions={versions}
            loading={query.isLoading}
            onSelect={(version) => setSelectedVersion(version)}
          />
          <ConfigVersionDrawer version={selectedVersion} onClose={() => setSelectedVersion(null)} />
        </div>
      </div>
    </AppShell>
  )
}
