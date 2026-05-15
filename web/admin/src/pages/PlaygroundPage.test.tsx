import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearToken, setToken } from '../lib/auth'
import { PlaygroundPage } from './PlaygroundPage'

describe('PlaygroundPage', () => {
  beforeEach(() => {
    setToken('demo-admin-token')
  })

  afterEach(() => {
    clearToken()
    vi.restoreAllMocks()
  })

  it('submits a chat completion request and renders response metadata', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'chatcmpl-1',
          object: 'chat.completion',
          model: 'gpt-4o-mini',
          choices: [
            {
              index: 0,
              message: {
                role: 'assistant',
                content: '这是一个测试响应。',
              },
              finish_reason: 'stop',
            },
          ],
          usage: {
            prompt_tokens: 10,
            completion_tokens: 20,
            total_tokens: 30,
          },
        }),
        {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
            'X-Cache': 'HIT',
            'X-Semantic-Score': '0.92',
          },
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    render(<PlaygroundPage />)

    await user.clear(screen.getByLabelText('Model'))
    await user.type(screen.getByLabelText('Model'), 'gpt-4o-mini')
    await user.click(screen.getByRole('button', { name: '发送请求' }))

    expect(await screen.findByText('Response JSON')).toBeInTheDocument()
    expect(screen.getByText('Request Preview')).toBeInTheDocument()
    expect(screen.getByText('HIT')).toBeInTheDocument()
    expect(screen.getByText('0.92')).toBeInTheDocument()
    expect(screen.getByText('200')).toBeInTheDocument()
    // Verify the URL includes the chat completions endpoint
    const fetchCall = fetchMock.mock.calls[0]
    expect(String(fetchCall[0])).toContain('/v1/chat/completions')
  })

  it('adds a new message row', async () => {
    const user = userEvent.setup()

    render(<PlaygroundPage />)

    expect(screen.getAllByLabelText('Role')).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: '添加消息' }))
    expect(screen.getAllByLabelText('Role')).toHaveLength(2)
  })
})
