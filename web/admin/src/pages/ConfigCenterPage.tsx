import { FormEvent, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { CreateDraftForm } from '../components/config/CreateDraftForm'
import { ConfigVersionDrawer } from '../components/config/ConfigVersionDrawer'
import { ConfigVersionTable } from '../components/config/ConfigVersionTable'
import { useConfigVersions } from '../hooks/useConfigVersions'
import { apiRequest, jsonRequest } from '../lib/http'
import type { ConfigVersion, ConfigVersionFilters, ConfigSnapshot } from '../types/admin'

const emptyFilters: ConfigVersionFilters = {
  module: '',
  tenantID: '',
  environment: '',
  scope: '',
  projectID: '',
}

export function ConfigCenterPage() {
  const queryClient = useQueryClient()
  const [draftFilters, setDraftFilters] = useState<ConfigVersionFilters>(emptyFilters)
  const [appliedFilters, setAppliedFilters] = useState<ConfigVersionFilters>(emptyFilters)
  const [selectedVersion, setSelectedVersion] = useState<ConfigVersion | null>(null)
  const [snapshotForm, setSnapshotForm] = useState({ version: '', config_snapshot: '', notes: '', show: false })
  const [importResult, setImportResult] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const query = useConfigVersions(appliedFilters)
  const versions = useMemo(() => query.data ?? [], [query.data])

  const snapshotsQuery = useQuery({
    queryKey: ['config-snapshots'],
    queryFn: () => apiRequest<{ object: string; data: ConfigSnapshot[] }>('/admin/config/versions'),
  })
  const snapshots = useMemo(() => snapshotsQuery.data?.data ?? [], [snapshotsQuery.data])

  const createSnapshotMutation = useMutation({
    mutationFn: (body: { version: string; config_snapshot: string; notes: string }) =>
      jsonRequest<ConfigSnapshot>('/admin/config/versions', body),
    onSuccess: () => {
      setSnapshotForm({ version: '', config_snapshot: '', notes: '', show: false })
      queryClient.invalidateQueries({ queryKey: ['config-snapshots'] })
    },
  })

  const publishMutation = useMutation({
    mutationFn: (id: number) => jsonRequest<{ status: string }>(`/admin/config/versions/${id}/publish`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['config-snapshots'] }),
  })

  const rollbackMutation = useMutation({
    mutationFn: (id: number) => jsonRequest<{ status: string }>(`/admin/config/versions/${id}/rollback`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['config-snapshots'] }),
  })

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setAppliedFilters({ ...draftFilters })
  }

  function handleExport() {
    const blob = new Blob([JSON.stringify({ object: 'list', data: snapshots }, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `config-snapshots-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function handleImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const parsed = JSON.parse(text)
      const data = parsed.data ?? parsed
      const result = await jsonRequest<{ imported: number }>('/admin/config/versions/import', { data })
      setImportResult(`已导入 ${result.imported} 条配置快照`)
      queryClient.invalidateQueries({ queryKey: ['config-snapshots'] })
    } catch (err) {
      setImportResult(`导入失败: ${(err as Error).message}`)
    }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  async function handleCreateSnapshot() {
    if (!snapshotForm.version.trim()) return
    createSnapshotMutation.mutate({
      version: snapshotForm.version.trim(),
      config_snapshot: snapshotForm.config_snapshot,
      notes: snapshotForm.notes.trim(),
    })
  }

  return (
    <AppShell
      title="配置中心"
      description="查看配置版本列表、管理配置快照、导入/导出配置。"
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

        <section className="page-surface" style={{ marginTop: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem', marginBottom: '1rem' }}>
            <div>
              <h2 style={{ margin: 0, fontSize: '1.1rem' }}>配置版本快照管理</h2>
              <p style={{ margin: '0.35rem 0 0', color: '#64748b' }}>创建草稿、发布、回滚、导入/导出</p>
            </div>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              {!snapshotForm.show ? (
                <button type="button" className="btn btn--primary" onClick={() => setSnapshotForm((p) => ({ ...p, show: true }))}>
                  创建草稿
                </button>
              ) : null}
              <button type="button" className="btn" onClick={handleExport} disabled={snapshots.length === 0}>
                导出
              </button>
              <button type="button" className="btn" onClick={() => fileInputRef.current?.click()}>
                导入
              </button>
              <input ref={fileInputRef} type="file" accept=".json" style={{ display: 'none' }} onChange={handleImport} />
            </div>
          </div>

          {importResult ? (
            <div className={importResult.includes('失败') ? 'config-error' : 'config-success'} style={{ marginBottom: '0.75rem' }}>
              {importResult}
            </div>
          ) : null}

          {snapshotForm.show ? (
            <div className="draft-form" style={{ marginBottom: '1rem' }}>
              <div className="draft-form__header">
                <div>
                  <h2>创建配置版本草稿</h2>
                  <p>输入完整 JSON 配置快照</p>
                </div>
                <button type="button" className="btn" onClick={() => setSnapshotForm((p) => ({ ...p, show: false }))}>
                  取消
                </button>
              </div>
              <div className="draft-form__grid">
                <label>
                  版本标识
                  <input
                    value={snapshotForm.version}
                    onChange={(e) => setSnapshotForm((p) => ({ ...p, version: e.target.value }))}
                    placeholder={`v${Date.now()}`}
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  配置快照 (JSON)
                  <textarea
                    rows={6}
                    value={snapshotForm.config_snapshot}
                    onChange={(e) => setSnapshotForm((p) => ({ ...p, config_snapshot: e.target.value }))}
                    placeholder='{"key": "value"}'
                    style={{ width: '100%', padding: '0.75rem 0.85rem', border: '1px solid #cbd5e1', borderRadius: '0.9rem', resize: 'vertical', fontFamily: 'monospace' }}
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  备注
                  <input
                    value={snapshotForm.notes}
                    onChange={(e) => setSnapshotForm((p) => ({ ...p, notes: e.target.value }))}
                    placeholder="初始配置"
                  />
                </label>
              </div>
              <button type="button" className="btn btn--primary" onClick={handleCreateSnapshot} disabled={createSnapshotMutation.isPending}>
                {createSnapshotMutation.isPending ? '创建中…' : '创建草稿'}
              </button>
              {createSnapshotMutation.error ? (
                <div className="config-error" style={{ marginTop: '0.5rem' }}>{(createSnapshotMutation.error as Error).message}</div>
              ) : null}
            </div>
          ) : null}

          {snapshotsQuery.isLoading ? (
            <div className="config-table__state">加载中…</div>
          ) : snapshots.length === 0 ? (
            <div className="config-table__state">暂无快照</div>
          ) : (
            <div className="config-table">
              <table>
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>版本</th>
                    <th>状态</th>
                    <th>备注</th>
                    <th>创建人</th>
                    <th>创建时间</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {[...snapshots].reverse().map((snap) => (
                    <tr key={snap.id}>
                      <td>{snap.id}</td>
                      <td style={{ fontWeight: 600 }}>{snap.version}</td>
                      <td>
                        <span className={`status-pill ${snap.status}`}>
                          {snap.status === 'draft' ? '草稿' : snap.status === 'published' ? '已发布' : '已回滚'}
                        </span>
                      </td>
                      <td style={{ color: '#64748b', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{snap.notes}</td>
                      <td>{snap.created_by}</td>
                      <td style={{ fontSize: '0.85rem', color: '#64748b' }}>{new Date(snap.created_at).toLocaleString()}</td>
                      <td>
                        <div style={{ display: 'flex', gap: '0.35rem' }}>
                          {snap.status === 'draft' ? (
                            <button type="button" className="btn btn--sm btn--primary" onClick={() => publishMutation.mutate(snap.id)} disabled={publishMutation.isPending}>
                              发布
                            </button>
                          ) : null}
                          {snap.status === 'published' ? (
                            <button type="button" className="btn btn--sm btn--danger-ghost" onClick={() => rollbackMutation.mutate(snap.id)} disabled={rollbackMutation.isPending}>
                              回滚
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </AppShell>
  )
}
