export type LedgerEntry = {
  id: number
  user_id: string
  amount: number
  type: string
  description: string
  reference_id: string
  created_at: string
}

export type PricingEntry = {
  provider: string
  model?: string
  input_price_per_1k: number
  output_price_per_1k: number
  is_default?: boolean
}

export type BalanceResponse = {
  balance: number
  currency: string
}
