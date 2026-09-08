import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { gameSocket, SESSION_REVOKED_CLOSE_CODE } from '@/services/gameSocket'

// Mock the browser WebSocket so we can drive close events.
class MockWebSocket {
  static instances: MockWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  url: string
  readyState = MockWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onclose: ((e: { code: number }) => void) | null = null
  onerror: (() => void) | null = null
  onmessage: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }
  close(): void {
    this.readyState = MockWebSocket.CLOSED
  }
  send(): void {}
}

vi.stubGlobal('WebSocket', MockWebSocket)

describe('gameSocket session revocation', () => {
  beforeEach(() => {
    vi.stubGlobal('WebSocket', MockWebSocket)
    MockWebSocket.instances = []
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    gameSocket.close()
  })

  it('notifies session-revoked listeners and does not reconnect on the revoked close code', () => {
    const revoked = vi.fn()
    const statuses: string[] = []
    gameSocket.onSessionRevoked(revoked)
    gameSocket.onStatus((s) => statuses.push(s))

    gameSocket.connect('ABC123')
    const ws = MockWebSocket.instances[0]
    ws.onopen?.()
    expect(statuses).toContain('connected')

    // Server terminates the connection because the session was revoked.
    ws.onclose?.({ code: SESSION_REVOKED_CLOSE_CODE })

    expect(revoked).toHaveBeenCalledTimes(1)
    // No reconnection must be scheduled: no new WebSocket is created.
    vi.advanceTimersByTime(60_000)
    expect(MockWebSocket.instances.length).toBe(1)
    expect(statuses).toContain('closed')
  })

  it('schedules a reconnect on a normal network drop', () => {
    const revoked = vi.fn()
    gameSocket.onSessionRevoked(revoked)

    gameSocket.connect('ABC123')
    const ws = MockWebSocket.instances[0]
    ws.onopen?.()
    ws.onclose?.({ code: 1006 }) // abnormal closure: treat as network drop

    expect(revoked).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1_000)
    expect(MockWebSocket.instances.length).toBe(2)
  })

  it('fires probe listeners after repeated failed reconnects (session may be revoked while down)', () => {
    const probe = vi.fn()
    gameSocket.onProbe(probe)

    gameSocket.connect('ABC123')
    const ws1 = MockWebSocket.instances[0]
    ws1.onopen?.()
    // First reconnect attempt: socket created, never opens, closes (1006).
    ws1.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(1_000)
    const ws2 = MockWebSocket.instances[1]
    ws2.onclose?.({ code: 1006 }) // attempt 2
    vi.advanceTimersByTime(2_000)
    const ws3 = MockWebSocket.instances[2]
    ws3.onclose?.({ code: 1006 }) // attempt 3
    vi.advanceTimersByTime(4_000)

    expect(probe).toHaveBeenCalled()
  })
})