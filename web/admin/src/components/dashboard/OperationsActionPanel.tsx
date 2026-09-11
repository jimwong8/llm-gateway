import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

export function OperationsActionPanel() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  const handleRefreshAll = () => {
    queryClient.invalidateQueries().then(() => {
      toast.success(t('panel.operations.refreshOk'))
    }).catch((err) => {
      toast.error(t('panel.operations.refreshFail'), { description: String(err) })
    })
  }

  const handleOpenPlayground = () => {
    navigate('/playground')
  }

  const handleExportErrors = () => {
    navigate('/audit-export')
  }

  const handleViewAudit = () => {
    navigate('/audit-runtime')
  }

  return (
    <section className="operations-action-panel" aria-label={t('panel.operations.aria')}>
      <header className="panel-header">
        <h2>{t('panel.operations.title')}</h2>
        <span>{t('panel.operations.subtitle')}</span>
      </header>
      <div className="operations-action-panel__grid">
        <button type="button" onClick={handleRefreshAll} aria-label={t('panel.operations.refreshAria')}>
          {t('panel.operations.refreshAll')}
        </button>
        <button type="button" onClick={handleOpenPlayground} aria-label={t('panel.operations.playgroundAria')}>
          {t('panel.operations.playground')}
        </button>
        <button type="button" onClick={handleExportErrors} aria-label={t('panel.operations.exportAria')}>
          {t('panel.operations.export')}
        </button>
        <button type="button" onClick={handleViewAudit} aria-label={t('panel.operations.auditAria')}>
          {t('panel.operations.audit')}
        </button>
      </div>
    </section>
  )
}
