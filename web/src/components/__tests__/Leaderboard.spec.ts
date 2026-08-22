import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import type { LeaderboardEntry } from '@/types/api'
import Leaderboard from '@/components/Leaderboard.vue'

describe('Leaderboard', () => {
  it('renders entries in the provided (sorted) order with correct ranks', () => {
    const entries: LeaderboardEntry[] = [
      { gamePlayerId: 'gp1', displayName: 'Alice', score: 12 },
      { gamePlayerId: 'gp2', displayName: 'Bob', score: 8 },
      { gamePlayerId: 'gp3', displayName: 'Carol', score: 5 },
    ]
    const wrapper = mount(Leaderboard, { props: { entries } })

    const names = wrapper.findAll('li span.flex-1').map((n) => n.text())
    expect(names).toEqual(['Alice', 'Bob', 'Carol'])

    const ranks = wrapper.findAll('li span.w-6').map((r) => r.text())
    expect(ranks).toEqual(['1', '2', '3'])
  })

  it('renders an empty list when no entries are provided', () => {
    const wrapper = mount(Leaderboard, { props: { entries: [] } })
    expect(wrapper.findAll('li')).toHaveLength(0)
  })
})