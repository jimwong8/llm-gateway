import type { FormEvent } from 'react'
import type { MemorySearchResult } from '../../types/memory'

interface MemorySearchPanelProps {
  searchQuery: string
  setSearchQuery: (q: string) => void
  searchTenantId: string
  setSearchTenantId: (v: string) => void
  searchUserId: string
  setSearchUserId: (v: string) => void
  searchResults: MemorySearchResult[]
  searchLoading: boolean
  searchError: string
  searchSubmitted: boolean
  t: (key: string, options?: Record<string, unknown>) => string
  onSearch: (event: FormEvent<HTMLFormElement>) => void
  onClear: () => void
}

export function MemorySearchPanel({
  searchQuery, setSearchQuery,
  searchTenantId, setSearchTenantId,
  searchUserId, setSearchUserId,
  searchResults, searchLoading, searchError, searchSubmitted,
  t, onSearch, onClear,
}: MemorySearchPanelProps) {
  return (
    <div className="memory-governance__search-panel">
      <form className="config-filters" aria-label={t('memory.searchFormLabel')} onSubmit={onSearch}>
        <label>
          {t('memory.searchContent')}
          <input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder={t('memory.searchPlaceholder')}
          />
        </label>
        <label>
          {t('memory.tenantId')}
          <input
            value={searchTenantId}
            onChange={(event) => setSearchTenantId(event.target.value)}
            placeholder={t('memory.optionalTenantScope')}
          />
        </label>
        <label>
          {t('memory.userId')}
          <input
            value={searchUserId}
            onChange={(event) => setSearchUserId(event.target.value)}
            placeholder={t('memory.optionalUserScope')}
          />
        </label>
        <div className="config-filters__actions">
          <button type="submit" disabled={searchLoading}>
            {searchLoading ? t('memory.searching') : t('memory.searchAction')}
          </button>
          <button
            type="button"
            className="rollouts-action"
            onClick={onClear}
          >
            {t('memory.clear')}
          </button>
        </div>
      </form>

      {searchError ? <div className="config-error">{searchError}</div> : null}

      {searchLoading ? (
        <div className="event-state">{t('memory.hybridSearching')}</div>
      ) : null}

      {!searchLoading && searchSubmitted && searchResults.length === 0 && !searchError ? (
        <div className="event-state">{t('memory.noSearchResults')}</div>
      ) : null}

      {!searchLoading && searchResults.length > 0 ? (
        <section className="event-table" aria-label={t('memory.searchResultsLabel')}>
          <div className="memory-governance__search-summary">
            {t('memory.searchResultCount', { count: searchResults.length })}
          </div>
          <table>
            <thead>
              <tr>
                <th className="memory-governance__rank-cell">{t('memory.rank')}</th>
                <th>{t('memory.content')}</th>
                <th className="memory-governance__score-cell">{t('memory.score')}</th>
                <th>{t('memory.source')}</th>
                <th>{t('memory.factKey')}</th>
                <th>{t('memory.tenant')}</th>
                <th>{t('memory.user')}</th>
              </tr>
            </thead>
            <tbody>
              {searchResults.map((result, index) => (
                <tr key={`${result.rank}-${result.source}-${index}`}>
                  <td className="memory-governance__rank-cell">
                    <span className="memory-governance__rank-badge">{result.rank}</span>
                  </td>
                  <td>
                    <div className="memory-fact-cell">
                      <strong>{result.content}</strong>
                    </div>
                  </td>
                  <td className="memory-governance__score-cell">
                    <span className="memory-governance__score-pill">{result.score.toFixed(4)}</span>
                  </td>
                  <td>{result.source}</td>
                  <td>{result.fact_key || '—'}</td>
                  <td>{result.tenant_id || '—'}</td>
                  <td>{result.user_id || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ) : null}
    </div>
  )
}
