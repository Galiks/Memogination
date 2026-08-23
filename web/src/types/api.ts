// API DTOs matching the backend projections (camelCase keys).

export interface ErrorContract {
  code: string
  message: string
  details?: Record<string, unknown>
  currentRevision?: number
}

export interface RoomDTO {
  id: string
  code: string
  state: string
  revision: number
}

export interface RoomSummary {
  id: string
  code: string
  state: string
  revision: number
  createdAt: string
  closedAt: string | null
  playerCount: number
}

export interface ScoreConfigDTO {
  allGuessedActivePlayer: number
  allGuessedGuesser: number
  noneGuessedActivePlayer: number
  noneGuessedOtherPlayer: number
  partialActiveBase: number
  partialActivePerGuesser: number
  partialGuesser: number
  voteForSubmittedMeme: number
}

export interface GameSettingsDTO {
  minPlayers: number
  maxPlayers: number
  handSize: number
  preparationTimeoutSeconds: number
  roundSelectionTimeoutSeconds: number
  votingTimeoutSeconds: number
  infiniteGame: boolean
  situationSeparator: string
  scoreConfig: ScoreConfigDTO
}

export interface GamePlayerDTO {
  id: string
  playerId: string
  displayName: string
  turnOrder: number
  score: number
  participationStatus: string
  connected: boolean
}

export interface RoomPlayerDTO {
  id: string
  name: string
  role: string
  connected: boolean
  isHost: boolean
}

export interface LeaderboardEntry {
  gamePlayerId: string
  displayName: string
  score: number
}

export interface GameDTO {
  id: string
  state: string
  currentCycleNumber: number
  settings: GameSettingsDTO
  players: GamePlayerDTO[]
  leaderboard: LeaderboardEntry[]
}

export interface ActorDTO {
  playerId: string
  name: string
  role: string
  isHost: boolean
  isAdmin: boolean
}

export interface GameSnapshot {
  revision: number
  serverTime: string
  room: RoomDTO
  game: GameDTO | null
  players: RoomPlayerDTO[]
  settings: GameSettingsDTO
  phase: string
  actor: ActorDTO
  phaseData: Record<string, unknown>
}

export interface GameEvent {
  type: string
  revision: number
  data?: Record<string, unknown>
}

export interface CommandRequest {
  commandId?: string
  expectedRevision?: number
  type: string
  payload?: Record<string, unknown>
}

export interface CommandResponse {
  events: GameEvent[]
  snapshot: GameSnapshot
}

export interface MemeDTO {
  id: string
  originalPath?: string
  screenPath: string
  thumbnailPath: string
  originalFilename: string
  mimeType: string
  sha256: string
  enabled: boolean
  source: string
  createdAt: string
}

export interface SituationDTO {
  id: string
  text: string
  enabled: boolean
  source: string
  createdAt: string
}

export interface HealthDTO {
  process_alive: boolean
  sqlite: boolean
  data_dir_writable: boolean
  media_dir_writable: boolean
}

export interface NetworkAddressesDTO {
  addresses: string[]
}

export interface BulkSituationsResult {
  found: number
  duplicates: number
  added: number
}