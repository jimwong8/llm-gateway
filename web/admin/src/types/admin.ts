export type VersionSource = {
  type: string
  source_environment: string
  source_version_id: string
}

export type ConfigVersion = {
  version_id: string
  status: string
  environment: string
  source?: VersionSource
}

export type ConfigVersionFilters = {
  module: string
  tenantID: string
  environment: string
  scope: string
  projectID: string
}
