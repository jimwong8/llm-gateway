import { useTranslation } from 'react-i18next'
import { AppShell } from '../components/layout/AppShell'
import { BatchActionBar } from '../components/memory/BatchActionBar'
import { BatchConfirmDialog } from '../components/memory/BatchConfirmDialog'
import { CandidateFactsTable } from '../components/memory/CandidateFactsTable'
import { FactDetailPanel } from '../components/memory/FactDetailPanel'
import { MemoryFilters } from '../components/memory/MemoryFilters'
import { MemorySearchPanel } from '../components/memory/MemorySearchPanel'
import { ProjectFactsTable } from '../components/memory/ProjectFactsTable'
import { useMemoryGovernance } from '../components/memory/useMemoryGovernance'
import { nextSortDirection } from '../components/memory/memoryUtils'

export function MemoryGovernancePage() {
  const { t: _t } = useTranslation()
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const t = _t as (key: string, options?: Record<string, unknown>) => string
  const g = useMemoryGovernance(t)

  const handleCandidateSort = (field: 'status' | 'updated_at') => {
    g.setCandidateSortDirection((previous) => nextSortDirection(g.candidateSortField, previous, field))
    g.setCandidateSortField(field)
  }

  const handleProjectSort = (field: 'status' | 'updated_at') => {
    g.setProjectSortDirection((previous) => nextSortDirection(g.projectSortField, previous, field))
    g.setProjectSortField(field)
  }

  const handleSearchClear = () => {
    g.setSearchQuery('')
    g.setSearchTenantId('')
    g.setSearchUserId('')
    g.setSearchResults([])
    g.setSearchError('')
    g.setSearchSubmitted(false)
  }

  return (
    <AppShell
      title={t('memory.pageTitle')}
      description={t('memory.pageDescription')}
    >
      <div className="events-page">
        <MemoryFilters
          draftFilters={g.draftFilters}
          setDraftFilters={g.setDraftFilters}
          draftCandidateStatus={g.draftCandidateStatus}
          setDraftCandidateStatus={g.setDraftCandidateStatus}
          draftProjectStatus={g.draftProjectStatus}
          setDraftProjectStatus={g.setDraftProjectStatus}
          t={t}
          onSubmit={g.handleSubmit}
          onReset={g.handleResetFilters}
        />

        <div className="memory-governance__tabs" role="tablist" aria-label={t('memory.tabSwitchLabel')}>
          <button
            type="button"
            role="tab"
            aria-selected={g.activeTab === 'governance'}
            className={`memory-governance__tab ${g.activeTab === 'governance' ? 'memory-governance__tab--active' : ''}`}
            onClick={() => g.setActiveTab('governance')}
          >
            {t('memory.tabGovernance')}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={g.activeTab === 'search'}
            className={`memory-governance__tab ${g.activeTab === 'search' ? 'memory-governance__tab--active' : ''}`}
            onClick={() => g.setActiveTab('search')}
          >
            {t('memory.tabSearch')}
          </button>
        </div>

        {g.activeTab === 'governance' ? (
          <>
            {g.candidateFactsQuery.isLoading || g.projectFactsQuery.isLoading ? (
              <div className="event-state">{t('memory.loadingFacts')}</div>
            ) : null}
            {g.candidateFactsQuery.error ? (
              <div className="config-error">{t('memory.candidateLoadError')}</div>
            ) : null}
            {g.projectFactsQuery.error ? (
              <div className="config-error">{t('memory.projectLoadError')}</div>
            ) : null}
            {g.actionError ? <div className="config-error">{g.actionError}</div> : null}
            {g.actionSuccess ? (
              <div className="event-state memory-governance__feedback" role="status" aria-live="polite">
                <strong>{t('memory.recentAction')}</strong>
                <div>{g.actionSuccess}</div>
                <div>{t('memory.autoRefreshed')}</div>
              </div>
            ) : null}
          </>
        ) : null}

        {g.activeTab === 'search' ? (
          <MemorySearchPanel
            searchQuery={g.searchQuery}
            setSearchQuery={g.setSearchQuery}
            searchTenantId={g.searchTenantId}
            setSearchTenantId={g.setSearchTenantId}
            searchUserId={g.searchUserId}
            setSearchUserId={g.setSearchUserId}
            searchResults={g.searchResults}
            searchLoading={g.searchLoading}
            searchError={g.searchError}
            searchSubmitted={g.searchSubmitted}
            t={t}
            onSearch={g.handleSearch}
            onClear={handleSearchClear}
          />
        ) : null}

        {g.activeTab === 'governance' && !g.candidateFactsQuery.isLoading && !g.projectFactsQuery.isLoading && !g.candidateFactsQuery.error && !g.projectFactsQuery.error ? (
          <>
            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('memory.candidateFacts')}</span>
                <strong>{g.metrics.totalCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.pending')}</span>
                <strong>{g.metrics.pendingCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.confirmed')}</span>
                <strong>{g.metrics.confirmedCandidates}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.promoted')}</span>
                <strong>{g.metrics.promotedCandidates}</strong>
                <small>{t('memory.rejectedCount', { count: g.metrics.rejectedCandidates })}</small>
              </section>
            </div>

            <div className="summary-card-grid">
              <section className="summary-card">
                <span>{t('memory.projectFacts')}</span>
                <strong>{g.metrics.totalProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.activeProjectFacts')}</span>
                <strong>{g.metrics.activeProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.supersededFacts')}</span>
                <strong>{g.metrics.supersededProjectFacts}</strong>
              </section>
              <section className="summary-card">
                <span>{t('memory.currentFilter')}</span>
                <strong>{g.candidateFilters.tenant_id || t('memory.allTenants')}</strong>
                <small>
                  {t('memory.filterSummary', {
                    user: g.candidateFilters.user_id || t('memory.allUsers'),
                    candidate: g.candidateFilters.status || t('memory.all'),
                    project: g.projectFilters.status || t('memory.all'),
                  })}
                </small>
              </section>
            </div>

            <div className="memory-governance__content">
              <div className="memory-governance__candidate-panel">
                <BatchActionBar
                  selectedCount={g.selectedVisibleCandidateFacts.length}
                  totalCount={g.pagedCandidateFacts.length}
                  filteredCount={g.filteredCandidateFacts.length}
                  fetchedCount={g.candidateFacts.length}
                  confirmable={g.selectedCandidateMetrics.confirm}
                  rejectable={g.selectedCandidateMetrics.reject}
                  promotable={g.selectedCandidateMetrics.promote}
                  batchActionSubmitting={g.batchActionSubmitting}
                  t={t}
                  onBatchAction={(action) => g.openBatchConfirmation(action, g.selectedVisibleCandidateFacts)}
                />

                <CandidateFactsTable
                  data={g.pagedCandidateFacts}
                  selectedFact={g.selectedFact?.kind === 'candidate' ? g.selectedFact : null}
                  selectedCandidateKeySet={g.selectedCandidateKeySet}
                  actionSubmitting={g.actionSubmitting}
                  batchActionSubmitting={g.batchActionSubmitting}
                  candidateSortField={g.candidateSortField}
                  candidateSortDirection={g.candidateSortDirection}
                  candidateLocalQuery={g.candidateLocalQuery}
                  setCandidateLocalQuery={g.setCandidateLocalQuery}
                  candidatePageSize={g.candidatePageSize}
                  setCandidatePageSize={g.setCandidatePageSize}
                  candidatePage={g.candidatePage}
                  setCandidatePage={g.setCandidatePage}
                  candidatePageCount={g.candidatePageCount}
                  candidatePagination={g.candidatePagination}
                  allVisibleSelected={g.allVisibleSelected}
                  selectAllVisibleRef={g.selectAllVisibleRef}
                  t={t}
                  onSort={handleCandidateSort}
                  onSelect={g.handleToggleCandidateSelection}
                  onToggleSelectAll={g.handleToggleSelectAllVisible}
                  onAction={g.handleCandidateAction}
                  onRowClick={(row) => g.setSelectedFact({ kind: 'candidate', fact: row })}
                />
              </div>

              <FactDetailPanel
                selectedFact={g.selectedFact}
                t={t}
              />

              <div className="memory-governance__project-panel">
                <ProjectFactsTable
                  data={g.pagedProjectFacts}
                  selectedFact={g.selectedFact?.kind === 'project' ? g.selectedFact : null}
                  projectSortField={g.projectSortField}
                  projectSortDirection={g.projectSortDirection}
                  projectLocalQuery={g.projectLocalQuery}
                  setProjectLocalQuery={g.setProjectLocalQuery}
                  projectPageSize={g.projectPageSize}
                  setProjectPageSize={g.setProjectPageSize}
                  projectPage={g.projectPage}
                  setProjectPage={g.setProjectPage}
                  projectPageCount={g.projectPageCount}
                  projectPagination={g.projectPagination}
                  t={t}
                  onSort={handleProjectSort}
                  onRowClick={(row) => g.setSelectedFact({ kind: 'project', fact: row })}
                />
              </div>
            </div>

            <div className="event-state memory-governance__hint">
              <strong>{t('memory.opsNote')}</strong>
              <div>{t('memory.candidateTableNote')}</div>
              <div>{t('memory.localSearchNote')}</div>
              <div>{t('memory.batchScopeNote')}</div>
            </div>

            {g.pendingBatchAction ? (
              <BatchConfirmDialog
                pendingBatchAction={g.pendingBatchAction}
                pendingBatchFacts={g.pendingBatchFacts}
                pendingBatchLabel={g.pendingBatchLabel}
                pendingBatchCount={g.pendingBatchCount}
                t={t}
                onCancel={() => { g.setPendingBatchAction(null); g.setPendingBatchFacts([]) }}
                onConfirm={() => {
                  const action = g.pendingBatchAction
                  const facts = g.pendingBatchFacts
                  g.setPendingBatchAction(null)
                  g.setPendingBatchFacts([])
                  if (action) {
                    void g.handleBatchCandidateAction(action, facts)
                  }
                }}
              />
            ) : null}
          </>
        ) : null}
      </div>
    </AppShell>
  )
}
