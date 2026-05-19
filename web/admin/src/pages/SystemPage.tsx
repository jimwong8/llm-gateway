import { FormEvent, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
      setSaveSuccess(t('system.saveSuccess'))
      queryClient.invalidateQueries({ queryKey: ['site-config'] })
    },
  })

  const rotateMutation = useMutation({
    mutationFn: () =>
      jsonRequest<{ jwt_secret: string; jwt_secret_rotated_at: string; message: string }>(
        '/admin/config/jwt/rotate', { updated_by: 'admin' },
      ),
    onSuccess: (data) => {
      setJwtResult(t('system.jwtRotated', { secret: data.jwt_secret, message: data.message }))
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
    <AppShell title={t('system.title')} description={t('system.description')}>
      <div className="system-page">
        {configQuery.error ? (
          <div className="config-error">{t('system.loadError', { message: (configQuery.error as Error).message })}</div>
        ) : null}

        <form className="page-surface" onSubmit={handleSubmit}>
          <h2 style={{ margin: '0 0 1rem', fontSize: '1.1rem' }}>{t('system.siteInfo')}</h2>
          <div className="system-config-grid">
            <label>
              {t('system.siteName')}
              <input value={form.site_name} onChange={(e) => updateField('site_name', e.target.value)} placeholder="LLM Gateway" />
            </label>
            <label>
              {t('system.logoUrl')}
              <input value={form.logo_url} onChange={(e) => updateField('logo_url', e.target.value)} placeholder="https://example.com/logo.png" />
            </label>
          </div>

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>{t('system.jwtSecurity')}</h2>
          <div className="system-config-grid">
            <div className="toggle-field">
              <span>{t('system.jwtStatus')}</span>
              <span className={cfg?.jwt_secret_configured ? 'badge badge--success' : 'badge badge--warning'}>
                {cfg?.jwt_secret_configured ? t('system.jwtConfigured') : t('system.jwtNotConfigured')}
              </span>
            </div>
            {cfg?.jwt_secret_rotated_at ? (
              <label>
                {t('system.lastRotatedAt')}
                <input value={cfg.jwt_secret_rotated_at} readOnly />
              </label>
            ) : null}
          </div>
          <div style={{ display: 'flex', gap: '0.75rem', marginTop: '0.75rem' }}>
            <button type="button" className="btn btn--primary" onClick={() => rotateMutation.mutate()} disabled={rotateMutation.isPending}>
              {rotateMutation.isPending ? t('system.rotating') : t('system.rotateJwt')}
            </button>
          </div>
          {jwtResult ? <pre className="system-jwt-result">{jwtResult}</pre> : null}
          {rotateMutation.error ? <div className="config-error">{(rotateMutation.error as Error).message}</div> : null}

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>{t('system.smtpConfig')}</h2>
          <div className="system-config-grid">
            <label>
              {t('system.smtpHost')}
              <input value={form.smtp_host} onChange={(e) => updateField('smtp_host', e.target.value)} placeholder="smtp.example.com" />
            </label>
            <label>
              {t('system.smtpPort')}
              <input type="number" value={form.smtp_port} onChange={(e) => updateField('smtp_port', parseInt(e.target.value) || 587)} placeholder="587" />
            </label>
            <label>
              {t('system.smtpUser')}
              <input value={form.smtp_user} onChange={(e) => updateField('smtp_user', e.target.value)} placeholder="user@example.com" />
            </label>
            <label>
              {t('system.smtpPass')}
              <input type="password" value={form.smtp_pass} onChange={(e) => updateField('smtp_pass', e.target.value)} placeholder={t('system.smtpPassPlaceholder')} />
            </label>
            <label>
              {t('system.smtpFrom')}
              <input value={form.smtp_from} onChange={(e) => updateField('smtp_from', e.target.value)} placeholder="noreply@example.com" />
            </label>
          </div>

          <h2 style={{ margin: '1.5rem 0 1rem', fontSize: '1.1rem' }}>{t('system.registrationQuota')}</h2>
          <div className="system-config-grid">
            <label className="toggle-field">
              <span>{t('system.allowRegistration')}</span>
              <input
                type="checkbox"
                checked={form.allow_registration}
                onChange={(e) => updateField('allow_registration', e.target.checked)}
              />
            </label>
            <label>
              {t('system.defaultUserRole')}
              <select value={form.default_user_role} onChange={(e) => updateField('default_user_role', e.target.value)}>
                <option value="user">user</option>
                <option value="admin">admin</option>
                <option value="readonly">readonly</option>
              </select>
            </label>
            <label>
              {t('system.defaultUserQuota')}
              <input type="number" value={form.default_user_quota} onChange={(e) => updateField('default_user_quota', parseInt(e.target.value) || 0)} placeholder="1000000" />
            </label>
          </div>

          <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1.5rem' }}>
            <button type="submit" className="btn btn--primary" disabled={saveMutation.isPending}>
              {saveMutation.isPending ? t('common.pending') : t('system.saveConfig')}
            </button>
          </div>
          {saveSuccess ? <div className="config-success" style={{ marginTop: '0.75rem' }}>{saveSuccess}</div> : null}
          {saveMutation.error ? <div className="config-error" style={{ marginTop: '0.75rem' }}>{(saveMutation.error as Error).message}</div> : null}
        </form>
      </div>
    </AppShell>
  )
}
