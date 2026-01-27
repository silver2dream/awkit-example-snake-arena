import React from 'react'
import { renderToString } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import App from './App'

describe('smoke', () => {
  it('renders App without crashing', () => {
    expect(() => renderToString(React.createElement(App))).not.toThrow()
  })
})
