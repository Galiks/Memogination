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
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
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
    vi.advanceTimersByTime(500)
    expect(MockWebSocket.instances.length).toBe(2)
  })

  it('applies deterministic jitter and caps exponential reconnect delays at ten seconds', () => {
    vi.mocked(Math.random).mockReturnValue(0.99)
    gameSocket.connect('JITTER1')
    const expectedDelays = [598, 1196, 2392, 4784, 9568, 10_000]
    expectedDelays.forEach((delay, attempt) => {
      MockWebSocket.instances[attempt].onclose?.({ code: 1006 })
      vi.advanceTimersByTime(delay - 1)
      expect(MockWebSocket.instances).toHaveLength(attempt + 1)
      vi.advanceTimersByTime(1)
      expect(MockWebSocket.instances).toHaveLength(attempt + 2)
    })
  })

  it('keeps the retry count after a short-lived reconnect', () => {
    gameSocket.connect('SHORT1')
    MockWebSocket.instances[0].onopen?.()
    MockWebSocket.instances[0].onclose?.({ code: 1006 })
    vi.advanceTimersByTime(500)
    const reconnected = MockWebSocket.instances[1]
    reconnected.onopen?.()
    reconnected.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(999)
    expect(MockWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(MockWebSocket.instances).toHaveLength(3)
  })

  it('resets exponential backoff only after a socket stays open for five seconds', () => {
    gameSocket.connect('STABLE1')
    MockWebSocket.instances[0].onopen?.()
    MockWebSocket.instances[0].onclose?.({ code: 1006 })
    vi.advanceTimersByTime(500)
    const reconnected = MockWebSocket.instances[1]
    reconnected.onopen?.()
    vi.advanceTimersByTime(5_000)
    reconnected.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(500)
    expect(MockWebSocket.instances).toHaveLength(3)
  })

  it('does not create a second socket for a duplicate connect', () => {
    gameSocket.connect('DUP123')
    gameSocket.connect('DUP123')
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('closes a prior room and ignores its late close callback', () => {
    gameSocket.connect('ROOMA')
    const oldSocket = MockWebSocket.instances[0]
    gameSocket.connect('ROOMB', { screen: true })
    oldSocket.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(20_000)
    expect(MockWebSocket.instances).toHaveLength(2)
    expect(MockWebSocket.instances[1].url).toContain('/ROOMB/ws?screen=1')
  })

  it('does not reconnect after a user closes the socket', () => {
    gameSocket.connect('CLOSE1')
    const socket = MockWebSocket.instances[0]
    gameSocket.close()
    socket.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(20_000)
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('fires probe listeners after repeated failed reconnects (session may be revoked while down)', () => {
    const probe = vi.fn()
    gameSocket.onProbe(probe)

    gameSocket.connect('ABC123')
    const ws1 = MockWebSocket.instances[0]
    ws1.onopen?.()
    // First reconnect attempt: socket created, never opens, closes (1006).
    ws1.onclose?.({ code: 1006 })
    vi.advanceTimersByTime(500)
    const ws2 = MockWebSocket.instances[1]
    ws2.onclose?.({ code: 1006 }) // attempt 2
    vi.advanceTimersByTime(1_000)
    const ws3 = MockWebSocket.instances[2]
    ws3.onclose?.({ code: 1006 }) // attempt 3
    vi.advanceTimersByTime(2_000)

    expect(probe).toHaveBeenCalled()
  })
})
