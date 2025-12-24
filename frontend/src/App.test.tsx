import { describe, expect, it } from 'vitest'
import { renderToString } from 'react-dom/server'
import App from './App'

describe('App', () => {
  it('renders the lobby header', () => {
    const html = renderToString(<App />)

    expect(html).toContain('Lobby')
    expect(html).toContain('Create room')
    expect(html).toContain('Join room')
  })
})
