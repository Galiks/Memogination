import type { LeaderboardEntry } from '@/types/api'

export interface PreparationPhaseData {
  preparedCount?: number
  totalPlayers?: number
  hand?: string[]
  preparedTurn?: { situationText: string; memeId: string }
}

export interface RoundSelectionPhaseData {
  situationText?: string
  activeGamePlayerId?: string
  hand?: string[]
  submitted?: boolean
}

export interface VoteOption {
  id: string
  number: number
  memeId: string
}

export interface RoundVotingPhaseData {
  situationText?: string
  voteOptions?: VoteOption[]
  forbiddenOptionId?: string
}

export interface RevealVoteOption {
  id: string
  number: number
  memeId: string
  ownerGamePlayerId: string
  isOriginal: boolean
  votes: number
}

export interface RevealSubmission {
  gamePlayerId: string
  displayName: string
  memeId: string
}

export interface RevealScoreDelta {
  gamePlayerId: string
  displayName: string
  delta: number
  newScore: number
}

export interface RevealData {
  situationText?: string
  originalMemeId?: string
  activeGamePlayerId?: string
  voteOptions?: RevealVoteOption[]
  submissions?: RevealSubmission[]
  scoreDeltas?: RevealScoreDelta[]
  leaderboard?: LeaderboardEntry[]
}

export interface RoundResultsPhaseData {
  reveal?: RevealData
}

export interface CycleResultsPhaseData {
  cycleNumber?: number
  leaderboard?: LeaderboardEntry[]
}

export interface GameResultsPhaseData {
  leaderboard?: LeaderboardEntry[]
}