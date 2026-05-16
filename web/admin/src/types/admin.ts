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
