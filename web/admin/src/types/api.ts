export interface ApiListResponse<T> {
  object: string
  data: T[]
  total?: number
  limit?: number
  offset?: number
}

export interface ApiResponse<T> {
  data: T
}

export interface ApiError {
  error: {
    message: string
    type: string
  }
}
