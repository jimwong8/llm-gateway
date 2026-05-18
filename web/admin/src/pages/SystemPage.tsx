import { FormEvent, useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AppShell } from '../components/layout/AppShell'
import { apiRequest, jsonRequest } from '../lib/http'
import type { SiteConfig } from '../types/admin'

type SiteConfigForm = {
  site_name: string
  logo_url: string
  smtp_host: string
  smtp_port: number
  smtp_user: string
  smtp_pass: string
  smtp_from: string
  allow_registration: boolean
  default_user_role: string
  default_user_quota: number
}

const emptyForm: SiteConfigForm = {
  site_name: '',
  logo_url: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_user: '',
  smtp_pass: '',
  smtp_from: '',
  allow_registration: true,
  default_user_role: 'user',
  default_user_quota: 1000000,
}

export function SystemPage() {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<SiteConfigForm>(emptyForm)
  const [jwtResult, setJwtResult] = useState('')
  const [saveSuccess, setSaveSuccess] = useState('')

  const configQuery = useQuery({
    queryKey: ['site-config'],
    queryFn: () => apiRequest<SiteConfig>('/admin/config/site'),
  })

  useEffect(() => {
    if (configQuery.data) {
      setForm({
        site_name: configQuery.data.site_name ?? '',
        logo_url: configQuery.data.logo_url ?? '',
        smtp_host: configQuery.data.smtp_host ?? '',
        smtp_port: configQuery.data.smtp_port ?? 587,
        smtp_user: configQuery.data.smtp_user ?? '',
        smtp_pass: '',
        smtp_from: configQuery.data.smtp_from ?? '',
        allow_registration: configQuery.data.allow_registration ?? true,
        default_user_role: configQuery.data.default_user_role ?? 'user',
        default_user_quota: configQuery.data.default_user_quota ?? 1000000,
      })
    }
  }, [configQuery.data])

  const saveMutation = useMutation({
    mutationFn: (data: SiteConfigForm) =>
      jsonRequest<SiteConfig>('/admin/config/site', {
        site_name: data.site_name,
        logo_url: data.logo_url,
        smtp_host: data.smtp_host,
        smtp_port: data.smtp_port,
        smtp_user: data.smtp_user,
        smtp_pass: data.smtp_pass,
        smtp_from: data.smtp_from,
        allow_registration: data.allow_registration,
        default_user_role: data.default_user_role,
        default_user_quota: data.default_user_quota,
        updated_by: 'admin',
      }, { method: 'PUT' } as RequestInit),
    onSuccess: () => {
      setSaveSuccess('站点配置已保存')
      queryClient.invalidateQueries({ queryKey: ['site-config'] })
    },
  })

  const rotateMutation = useMutation({
    mutationFn: () =>
      jsonRequest<{ jwt_secret: string; jwt_secret_rotated_at: string; message: string }>(
        '/admin/config/jwt/rotate', { updated_by: 'admin' },
      ),
    onSuccess: (data) => {
      setJwtResult(`新 Secret: ${data.jwt_secret}\n${data.message}`)
      queryClient.invalidateQueries({ queryKey: ['site-config'] })
    },
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaveSuccess('')
    setJwtResult('')
    saveMutation.mutate(form)
  }

  function updateField<K extends keyof SiteConfigForm>(key: K, value: SiteConfigForm[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const cfg = configQuery.data

  return (
    <AppShell title="系统设置" description="管理系统站点配置、JWT Secret、SMTP 邮件、注册开关等。">
      <div className="system-page">
        {configQuery.error ? (
          <div className="config-error">加载配置失败: {(configQuery.error as Error).message}</div>
        ) : null}

        <form className="page-surface" onSubmit={handleSubmit}>
          <h2 style={{ margin: '0 0 1rem', fontSize: '1.1rem' }}>站点信息</h2>
          <div className="system-config-grid">
            <label>
              站点名称
              <input value={form.site_name} onChange={(e) => updateField('site_name', e.target.value)} placeholder="LLM Gateway" />
            </label>
            <label>
              Logo URL
              <input value={form.logo_url} onChange={(e) => updateField('logo_url', e.target.value)} placeholder="https://example.com/logo.png" />
            </label>
          </div>

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>JWT 安全</h2>
          <div className="system-config-grid">
            <div className="toggle-field">
              <span>JWT Secret 状态</span>
              <span className={cfg?.jwt_secret_configured ? 'badge badge--success' : 'badge badge--warning'}>
                {cfg?.jwt_secret_configured ? '已配置' : '未配置'}
              </span>
            </div>
            {cfg?.jwt_secret_rotated_at ? (
              <label>
                上次轮换时间
                <input value={cfg.jwt_secret_rotated_at} readOnly />
              </label>
            ) : null}
          </div>
          <div style={{ display: 'flex', gap: '0.75rem', marginTop: '0.75rem' }}>
            <button type="button" className="btn btn--primary" onClick={() => rotateMutation.mutate()} disabled={rotateMutation.isPending}>
              {rotateMutation.isPending ? '轮换中…' : '重新生成 JWT Secret'}
            </button>
          </div>
          {jwtResult ? <pre className="system-jwt-result">{jwtResult}</pre> : null}
          {rotateMutation.error ? <div className="config-error">{(rotateMutation.error as Error).message}</div> : null}

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>SMTP 邮件配置</h2>
          <div className="system-config-grid">
            <label>
              SMTP 主机
              <input value={form.smtp_host} onChange={(e) => updateField('smtp_host', e.target.value)} placeholder="smtp.example.com" />
            </label>
            <label>
              端口
              <input type="number" value={form.smtp_port} onChange={(e) => updateField('smtp_port', parseInt(e.target.value) || 587)} placeholder="587" />
            </label>
            <label>
              用户名
              <input value={form.smtp_user} onChange={(e) => updateField('smtp_user', e.target.value)} placeholder="user@example.com" />
            </label>
            <label>
              密码
              <input type="password" value={form.smtp_pass} onChange={(e) => updateField('smtp_pass', e.target.value)} placeholder="留空则不变" />
            </label>
            <label>
              发件人地址
              <input value={form.smtp_from} onChange={(e) => updateField('smtp_from', e.target.value)} placeholder="noreply@example.com" />
            </label>
          </div>

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>注册与配额</h2>
          <div className="system-config-grid">
            <label className="toggle-field">
              <span>允许新用户注册</span>
              <input
                type="checkbox"
                checked={form.allow_registration}
                onChange={(e) => updateField('allow_registration', e.target.checked)}
              />
            </label>
            <label>
              默认用户角色
              <select value={form.default_user_role} onChange={(e) => updateField('default_user_role', e.target.value)}>
                <option value="user">user</option>
                <option value="admin">admin</option>
                <option value="readonly">readonly</option>
              </select>
            </label>
            <label>
              默认用户配额（token/月）
              <input type="number" value={form.default_user_quota} onChange={(e) => updateField('default_user_quota', parseInt(e.target.value) || 0)} placeholder="1000000" />
            </label>
          </div>

          <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1.5rem' }}>
            <button type="submit" className="btn btn--primary" disabled={saveMutation.isPending}>
              {saveMutation.isPending ? '保存中…' : '保存配置'}
            </button>
          </div>
          {saveSuccess ? <div className="config-success" style={{ marginTop: '0.75rem' }}>{saveSuccess}</div> : null}
          {saveMutation.error ? <div className="config-error" style={{ marginTop: '0.75rem' }}>{(saveMutation.error as Error).message}</div> : null}
        </form>
      </div>
    </AppShell>
  )
}
