/** 配置版本来源 */
export type VersionSource = {
  type: string       // 来源类型
  source_environment: string  // 来源环境
  source_version_id: string   // 来源版本 ID
}

/** 配置版本 */
export type ConfigVersion = {
  version_id: string    // 版本唯一标识
  status: string        // 状态：draft / released / promoted
  environment: string   // 目标环境
  source?: VersionSource  // 继承来源信息
}

/** 配置版本筛选条件 */
export type ConfigVersionFilters = {
  module: string      // 模块名
  tenantID: string    // 租户 ID
  environment: string // 环境
  scope: string       // 作用域
  projectID: string   // 项目 ID
}

/** 站点配置 */
export type SiteConfig = {
  site_name: string
  logo_url: string
  jwt_secret_configured?: boolean
  jwt_secret_rotated_at?: string
  smtp_host: string
  smtp_port: number
  smtp_user: string
  smtp_from: string
  allow_registration: boolean
  default_user_role: string
  default_user_quota: number
  updated_at: string
  updated_by: string
}

/** 配置快照 */
export type ConfigSnapshot = {
  id: number
  version: string
  status: 'draft' | 'published' | 'rolled_back'
  config_snapshot?: string
  notes: string
  created_by: string
  created_at: string
  published_at?: string
  rolled_back_at?: string
}
