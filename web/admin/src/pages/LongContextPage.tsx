import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  cancelLongContextTask,
  createLongContextTask,
  fetchLongContextHealth,
  fetchLongContextTasks,
  runLongContextTask,
} from '../lib/api/longcontext'
import { Button } from '../components/ui'
import { toast } from 'sonner'

const statusClass: Record<string, string> = {
  queued: 'status-badge queued',
  ingesting: 'status-badge ingesting',
  mapping: 'status-badge mapping',
  indexing: 'status-badge indexing',
  retrieving: 'status-badge retrieving',
  synthesizing: 'status-badge synthesizing',
  verifying: 'status-badge verifying',
  succeeded: 'status-badge succeeded',
  failed: 'status-badge failed',
  cancelled: 'status-badge cancelled',
}

export function LongContextPage() {
  const queryClient = useQueryClient()
  const [tenantID, setTenantID] = useState('default')
  const [selected, setSelected] = useState<string | null>(null)

  const { data: health } = useQuery({
    queryKey: ['longcontext-health'],
    queryFn: fetchLongContextHealth,
    refetchInterval: 30000,
  })

  const { data, isLoading } = useQuery({
    queryKey: ['longcontext-tasks', tenantID],
    queryFn: () => fetchLongContextTasks({ tenant_id: tenantID, limit: 50 }),
    refetchInterval: 5000,
  })

  const createMutation = useMutation({
    mutationFn: createLongContextTask,
    onSuccess: () => {
      toast.success('任务已创建')
      queryClient.invalidateQueries({ queryKey: ['longcontext-tasks'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const runMutation = useMutation({
    mutationFn: runLongContextTask,
    onSuccess: () => {
      toast.success('任务已开始执行')
      queryClient.invalidateQueries({ queryKey: ['longcontext-tasks'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const cancelMutation = useMutation({
    mutationFn: ({ id, tenantID }: { id: string; tenantID: string }) => cancelLongContextTask(id, tenantID),
    onSuccess: () => {
      toast.success('任务已取消')
      queryClient.invalidateQueries({ queryKey: ['longcontext-tasks'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const selectedTask = data?.tasks.find((t) => t.id === selected)

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">虚拟长上下文任务</h1>
        <div className="flex items-center gap-3">
          <span className={health?.status === 'healthy' ? 'text-green-500' : 'text-red-500'}>
            {health ? `健康: ${health.db}` : '检查中...'}
          </span>
          <Button
            onClick={() =>
              createMutation.mutate({
                model: 'virtual-long-1m',
                mode: 'qa',
                query: '总结这份文档的要点。',
                input: { text: '在此粘贴需要处理的长文本...' },
                tenant_id: tenantID,
              })
            }
          >
            创建示例任务
          </Button>
        </div>
      </div>

      <div className="flex gap-4">
        <input
          className="input"
          placeholder="tenant_id"
          value={tenantID}
          onChange={(e) => setTenantID(e.target.value)}
        />
      </div>

      {isLoading ? (
        <p>加载中...</p>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-1 card">
            <h2 className="font-semibold mb-3">任务列表</h2>
            <div className="space-y-2 max-h-[70vh] overflow-auto">
              {(data?.tasks ?? []).map((task) => (
                <button
                  key={task.id}
                  type="button"
                  onClick={() => setSelected(task.id)}
                  className={`w-full text-left p-3 rounded border ${selected === task.id ? 'border-primary bg-primary/10' : 'border-border'}`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-xs truncate max-w-[140px]">{task.id}</span>
                    <span className={statusClass[task.status] ?? 'status-badge'}>{task.status}</span>
                  </div>
                  <div className="text-sm text-muted-foreground mt-1 truncate">{task.query}</div>
                  <div className="text-xs text-muted-foreground mt-1">
                    chunks {task.completed_chunks}/{task.chunk_count} · retries {task.retry_count}
                  </div>
                </button>
              ))}
            </div>
          </div>

          <div className="lg:col-span-2 card">
            {selectedTask ? (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h2 className="font-semibold">任务详情</h2>
                  <div className="flex gap-2">
                    {['queued', 'mapping', 'ingesting'].includes(selectedTask.status) && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => cancelMutation.mutate({ id: selectedTask.id, tenantID })}
                      >
                        取消
                      </Button>
                    )}
                    {['queued', 'failed'].includes(selectedTask.status) && (
                      <Button
                        size="sm"
                        onClick={() =>
                          runMutation.mutate({
                            task_id: selectedTask.id,
                            tenant_id: tenantID,
                            channel: 'nvidia-nim-key-10',
                            model: 'deepseek-ai/deepseek-v4-pro-0813',
                            reader_channel: 'nvidia-nim-key-10',
                            reader_model: 'deepseek-ai/deepseek-v4-pro-0813',
                            reader_top_k: 4,
                          })
                        }
                      >
                        执行
                      </Button>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>状态: <span className={statusClass[selectedTask.status]}>{selectedTask.status}</span></div>
                  <div>阶段: {selectedTask.phase}</div>
                  <div>Chunks: {selectedTask.completed_chunks}/{selectedTask.chunk_count}</div>
                  <div>Evidence: {selectedTask.evidence_count ?? 0}</div>
                  <div>Retrieved: {selectedTask.retrieved_count ?? 0}</div>
                  <div>Coverage: {selectedTask.citation_coverage ?? 0}</div>
                  <div>Tokens: {selectedTask.used_tokens}</div>
                  <div>Estimated cost: {selectedTask.estimated_cost}</div>
                </div>

                {selectedTask.last_error ? (
                  <div className="p-3 bg-red-500/10 rounded text-red-400 text-sm">
                    {selectedTask.last_error}
                  </div>
                ) : null}

                {selectedTask.result ? (
                  <div className="space-y-2">
                    <h3 className="font-semibold">最终答案</h3>
                    <div className="p-3 rounded bg-muted whitespace-pre-wrap text-sm">{selectedTask.result.answer}</div>
                    <div className="text-xs text-muted-foreground">
                      引用: {selectedTask.result.cited_evidence_ids.join(', ')}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : (
              <p className="text-muted-foreground">请选择左侧任务查看详情</p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
