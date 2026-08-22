import { defineStore } from 'pinia'
import { ref } from 'vue'

export type UiTab = 'memes' | 'situations' | 'settings'

export const useUiStore = defineStore('ui', () => {
  const selectedMemeId = ref<string | null>(null)
  const situationText = ref('')
  const currentTab = ref<UiTab>('memes')
  const showSettingsModal = ref(false)
  const showUploadModal = ref(false)
  const showBulkModal = ref(false)

  function openSettings(): void {
    showSettingsModal.value = true
  }

  function closeSettings(): void {
    showSettingsModal.value = false
  }

  function openUpload(): void {
    showUploadModal.value = true
  }

  function closeUpload(): void {
    showUploadModal.value = false
  }

  function openBulk(): void {
    showBulkModal.value = true
  }

  function closeBulk(): void {
    showBulkModal.value = false
  }

  return {
    selectedMemeId,
    situationText,
    currentTab,
    showSettingsModal,
    showUploadModal,
    showBulkModal,
    openSettings,
    closeSettings,
    openUpload,
    closeUpload,
    openBulk,
    closeBulk,
  }
})