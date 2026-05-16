/** 通用列表响应封装 */
export type ListResponse<T> = {
  object: string   // 对象类型标识
  data: T[]        // 数据列表
}
