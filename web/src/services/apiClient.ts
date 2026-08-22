import type {
  BulkSituationsResult,
  CommandRequest,
  CommandResponse,
  ErrorContract,
  GameSettingsDTO,
  GameSnapshot,
  HealthDTO,
  MemeDTO,
  NetworkAddressesDTO,
  SituationDTO,
} from '@/types/api'

const BASE_URL = '/api/v1'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, unknown>
  readonly currentRevision?: number

  constructor(status: number, contract: ErrorContract) {
    super(contract.message || `Request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = contract.code
    this.details = contract.details
    this.currentRevision = contract.currentRevision
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  let response: Response
  try {
    response = await fetch(`${BASE_URL}${path}`, {
      ...init,
      headers,
      credentials: 'same-origin',
    })
  } catch (err) {
    throw new ApiError(0, {
      code: 'NETWORK_ERROR',
      message: err instanceof Error ? err.message : 'Network error',
    })
  }

  if (!response.ok) {
    let contract: ErrorContract = { code: 'UNKNOWN', message: response.statusText }
    try {
      const body = await response.json()
      contract = body as ErrorContract
    } catch {
      // fall back to the default contract
    }
    throw new ApiError(response.status, contract)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

function jsonBody(data: unknown): RequestInit {
  return { body: JSON.stringify(data) }
}

export const apiClient = {
  // Rooms
  joinRoom(code: string, name: string): Promise<GameSnapshot> {
    return request(`/rooms/${encodeURIComponent(code)}/join`, {
      method: 'POST',
      ...jsonBody({ name }),
    })
  },
  reconnect(code: string): Promise<GameSnapshot> {
    return request(`/rooms/${encodeURIComponent(code)}/reconnect`, { method: 'POST' })
  },
  createRoom(name: string): Promise<{ id: string; code: string; revision: number; state: string; createdAt: string }> {
    return request('/rooms', { method: 'POST', ...jsonBody({ name }) })
  },
  getState(code: string, screen = false): Promise<GameSnapshot> {
    const query = screen ? '?screen=1' : ''
    return request(`/rooms/${encodeURIComponent(code)}/state${query}`)
  },
  getSettings(code: string): Promise<GameSettingsDTO> {
    return request(`/rooms/${encodeURIComponent(code)}/settings`)
  },
  updateSettings(code: string, settings: GameSettingsDTO): Promise<GameSettingsDTO> {
    return request(`/rooms/${encodeURIComponent(code)}/settings`, {
      method: 'PUT',
      ...jsonBody(settings),
    })
  },
  sendCommand(code: string, command: CommandRequest): Promise<CommandResponse> {
    return request(`/rooms/${encodeURIComponent(code)}/commands`, {
      method: 'POST',
      ...jsonBody(command),
    })
  },

  // Memes
  listMemes(): Promise<MemeDTO[]> {
    return request('/memes')
  },
  uploadMeme(file: File): Promise<MemeDTO> {
    const form = new FormData()
    form.append('file', file)
    return request('/memes', { method: 'POST', body: form })
  },
  deleteMeme(id: string): Promise<void> {
    return request(`/memes/${encodeURIComponent(id)}`, { method: 'DELETE' })
  },
  updateMeme(id: string, patch: { enabled?: boolean }): Promise<MemeDTO> {
    return request(`/memes/${encodeURIComponent(id)}`, { method: 'PATCH', ...jsonBody(patch) })
  },

  // Situations
  listSituations(): Promise<SituationDTO[]> {
    return request('/situations')
  },
  addSituation(text: string, enabled = true): Promise<SituationDTO> {
    return request('/situations', { method: 'POST', ...jsonBody({ text, enabled }) })
  },
  bulkAddSituations(text: string, delimiter = '*'): Promise<BulkSituationsResult> {
    return request('/situations/bulk', { method: 'POST', ...jsonBody({ text, delimiter }) })
  },
  deleteSituation(id: string): Promise<void> {
    return request(`/situations/${encodeURIComponent(id)}`, { method: 'DELETE' })
  },
  updateSituation(id: string, patch: { text?: string; enabled?: boolean }): Promise<SituationDTO> {
    return request(`/situations/${encodeURIComponent(id)}`, { method: 'PATCH', ...jsonBody(patch) })
  },

  // Admin / misc
  adminBootstrap(): Promise<{ isAdmin: boolean }> {
    return request('/admin/bootstrap', { method: 'POST' })
  },
  getNetworkAddresses(): Promise<NetworkAddressesDTO> {
    return request('/network/addresses')
  },
  health(): Promise<HealthDTO> {
    return request('/health')
  },
}