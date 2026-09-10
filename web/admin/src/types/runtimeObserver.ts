export type RuntimeObserverActivePolicy = {
  version_id: string
  updated_at?: string
}

export type RuntimeObserverCacheEntry = {
  environment: string
  policy_version_id: string
  cached_at?: string
}

export type RuntimeObserverCacheState = {
  entry_count: number
  entries: RuntimeObserverCacheEntry[]
  invalidation_count: number
  last_invalidated_at?: string
  last_invalidated_environment?: string
}

export type RuntimeObserverDecisionFact = {
  request_id: string
  policy_version_id: string
  rollout_id?: string
  resolved_model: string
  matched_scope_type?: string
  success?: boolean
  created_at?: string
}

export type RuntimeObserverDistributionFact = {
  event_id: string
  policy_version_id?: string
  rollout_id?: string
  event_type: string
  payload?: Record<string, unknown>
  created_at?: string
}

export type RuntimeObserverFacts = {
  runtime_decisions: RuntimeObserverDecisionFact[]
  distribution_events: RuntimeObserverDistributionFact[]
}

export type RuntimeObserverResponse = {
  environment: string
  observed_at?: string
  active_policy: RuntimeObserverActivePolicy
  cache: RuntimeObserverCacheState
  facts: RuntimeObserverFacts
}
