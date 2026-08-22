import { computed, ref } from 'vue'
import type { MemeDTO } from '@/types/api'
import { apiClient } from '@/services/apiClient'

/**
 * Loads the meme catalog (public endpoint) and provides id -> MemeDTO lookup.
 * Used by player/screen views to render meme images from bare meme ids.
 */
export function useMemeCatalog() {
  const memes = ref<MemeDTO[]>([])
  const loading = ref(false)
  const error = ref('')

  const byId = computed(() => {
    const map = new Map<string, MemeDTO>()
    for (const m of memes.value) map.set(m.id, m)
    return map
  })

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      memes.value = await apiClient.listMemes()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Не удалось загрузить мемы'
    } finally {
      loading.value = false
    }
  }

  function get(id?: string): MemeDTO | null {
    if (!id) return null
    return byId.value.get(id) ?? null
  }

  return { memes, loading, error, load, get }
}