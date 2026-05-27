import '@testing-library/jest-dom'
import '../i18n'

// Node 24 + jsdom 下，react-router 构造 Request 时会把 jsdom realm 的 AbortSignal
// 传给 undici realm 的 RequestInit，触发类型检查异常。
// 测试环境中去掉 RequestInit.signal（导航取消信号）即可稳定路由测试。
const NativeRequest = globalThis.Request

class TestRequest extends NativeRequest {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    if (init && Object.prototype.hasOwnProperty.call(init, 'signal')) {
      const { signal: _ignoredSignal, ...rest } = init
      super(input, rest)
      return
    }
    super(input, init)
  }
}

globalThis.Request = TestRequest as typeof Request
