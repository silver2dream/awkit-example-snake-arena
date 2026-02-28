import { createElement } from 'react'
import { renderToString } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import App from './App'

describe('smoke', () => {
  it('renders App without errors', () => {
    const html = renderToString(createElement(App))

    expect(html).toContain('Snake Arena')
    expect(html).toContain('Connection: <!-- -->disconnected')
    expect(html).toContain('Create Room')
    expect(html).toContain('Join Room')
    expect(html).toContain('Available Rooms')
  })
})
