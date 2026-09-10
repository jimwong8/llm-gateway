import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export function OperationsActionPanel() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const handleRefreshAll = () => {
    queryClient.invalidateQueries().then(() => {
      toast.success('已触发全局刷新')
    }).catch((err) => {
      toast.error('刷新失败', { description: String(err) })
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
    <section className="operations-action-panel" aria-label="运维快捷动作">
      <header className="panel-header">
        <h2>快捷操作</h2>
        <span>运维工具</span>
      </header>
      <div className="operations-action-panel__grid">
        <button type="button" onClick={handleRefreshAll} aria-label="刷新全部仪表盘数据">
          刷新全部
        </button>
        <button type="button" onClick={handleOpenPlayground} aria-label="打开在线测试 Playground">
          打开在线测试
        </button>
        <button type="button" onClick={handleExportErrors} aria-label="导出错误日志">
          导出错误日志
        </button>
        <button type="button" onClick={handleViewAudit} aria-label="查看审计日志">
          查看审计导出
        </button>
      </div>
    </section>
  )
}
