import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import { DOMWrapper, mount, type VueWrapper } from '@vue/test-utils'
import type { MemeDTO } from '@/types/api'
import MemeSlider from '@/components/MemeSlider.vue'

function meme(id: string): MemeDTO {
  return {
    id,
    originalPath: `${id}.jpg`,
    screenPath: `${id}.screen.jpg`,
    thumbnailPath: `${id}.thumb.jpg`,
    originalFilename: `${id}.jpg`,
    mimeType: 'image/jpeg',
    sha256: id,
    enabled: true,
    source: 'upload',
    createdAt: '2026-01-01T00:00:00Z',
  }
}

const memes = [meme('m1'), meme('m2'), meme('m3')]

let wrapper: VueWrapper | undefined

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})

function mountSlider(props: Record<string, unknown> = {}): VueWrapper {
  wrapper = mount(MemeSlider, { props: { memes, ...props } })
  return wrapper
}

// The overlay is teleported to body, so it must be reached through the DOM.
function overlay(): DOMWrapper<HTMLElement> {
  const el = document.body.querySelector('[data-testid="meme-fullscreen-overlay"]')
  if (!el) throw new Error('fullscreen overlay not found')
  return new DOMWrapper(el as HTMLElement)
}

describe('MemeSlider', () => {
  it('renders the current slide with the screen image, counter and thumbnails', () => {
    const w = mountSlider()
    const slide = w.find('[data-testid="meme-option"]')
    expect(slide.exists()).toBe(true)
    expect(slide.find('img').attributes('src')).toBe('/media/m1.screen.jpg')
    expect(w.text()).toContain('1 / 3')
    expect(w.findAll('[data-testid="meme-slide-nav"]')).toHaveLength(3)
  })

  it('starts on the selected meme when it is present in the list', () => {
    const w = mountSlider({ selectedId: 'm3' })
    expect(w.text()).toContain('3 / 3')
    expect(w.find('[data-testid="meme-option"] img').attributes('src')).toBe('/media/m3.screen.jpg')
  })

  it('emits select with the current meme id on slide click', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-option"]').trigger('click')
    expect(w.emitted('select')).toEqual([['m1']])
  })

  it('navigates with next/prev arrows and wraps around', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-next"]').trigger('click')
    expect(w.text()).toContain('2 / 3')
    await w.find('[data-testid="meme-next"]').trigger('click')
    expect(w.text()).toContain('3 / 3')
    // Wrap-around forward
    await w.find('[data-testid="meme-next"]').trigger('click')
    expect(w.text()).toContain('1 / 3')
    // Wrap-around backward
    await w.find('[data-testid="meme-prev"]').trigger('click')
    expect(w.text()).toContain('3 / 3')
  })

  it('navigates to the tapped dot index', async () => {
    const w = mountSlider()
    const dots = w.findAll('[data-testid="meme-dot"]')
    expect(dots).toHaveLength(3)
    await dots[2].trigger('click')
    expect(w.text()).toContain('3 / 3')
    await dots[0].trigger('click')
    expect(w.text()).toContain('1 / 3')
  })

  it('navigates to the clicked thumbnail', async () => {
    const w = mountSlider()
    await w.findAll('[data-testid="meme-slide-nav"]')[1].trigger('click')
    expect(w.text()).toContain('2 / 3')
  })

  it('resets to the first meme when the list is replaced without a matching selection', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-next"]').trigger('click')
    expect(w.text()).toContain('2 / 3')

    const newMemes = [meme('x1'), meme('x2')]
    await w.setProps({ memes: newMemes })
    expect(w.text()).toContain('1 / 2')
    expect(w.find('[data-testid="meme-option"] img').attributes('src')).toBe('/media/x1.screen.jpg')
  })

  it('opens the fullscreen overlay and close hides it', async () => {
    const w = mountSlider()
    expect(document.body.querySelector('[data-testid="meme-fullscreen-close"]')).toBeNull()

    await w.find('[data-testid="meme-fullscreen"]').trigger('click')
    const overlayImage = document.body.querySelector('[data-testid="meme-fullscreen-image"]')
    expect(overlayImage).not.toBeNull()
    expect((overlayImage as HTMLImageElement).getAttribute('src')).toBe('/media/m1.screen.jpg')

    const close = document.body.querySelector('[data-testid="meme-fullscreen-close"]') as HTMLButtonElement
    close.click()
    await nextTick()
    expect(document.body.querySelector('[data-testid="meme-fullscreen-close"]')).toBeNull()
  })

  it('navigates with the fullscreen overlay prev/next buttons', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-fullscreen"]').trigger('click')

    const next = document.body.querySelector('[data-testid="meme-fullscreen-next"]') as HTMLButtonElement
    next.click()
    await nextTick()
    expect(
      (document.body.querySelector('[data-testid="meme-fullscreen-image"]') as HTMLImageElement).getAttribute('src'),
    ).toBe('/media/m2.screen.jpg')

    const prev = document.body.querySelector('[data-testid="meme-fullscreen-prev"]') as HTMLButtonElement
    prev.click()
    await nextTick()
    expect(
      (document.body.querySelector('[data-testid="meme-fullscreen-image"]') as HTMLImageElement).getAttribute('src'),
    ).toBe('/media/m1.screen.jpg')
  })

  it('closes the fullscreen overlay with Escape', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-fullscreen"]').trigger('click')
    expect(document.body.querySelector('[data-testid="meme-fullscreen-close"]')).not.toBeNull()

    await overlay().trigger('keydown', { key: 'Escape' })
    expect(document.body.querySelector('[data-testid="meme-fullscreen-close"]')).toBeNull()
  })

  it('swipe then immediately click close still closes the fullscreen overlay', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-fullscreen"]').trigger('click')

    const ov = overlay()
    await ov.trigger('touchstart', { touches: [{ clientX: 300, clientY: 100 }] })
    await ov.trigger('touchend', { changedTouches: [{ clientX: 100, clientY: 110 }] })

    const close = document.body.querySelector('[data-testid="meme-fullscreen-close"]') as HTMLButtonElement
    close.click()
    await nextTick()
    expect(document.body.querySelector('[data-testid="meme-fullscreen-close"]')).toBeNull()
  })

  it('navigates with ArrowRight key on the slide', async () => {
    const w = mountSlider()
    await w.find('[data-testid="meme-option"]').trigger('keydown', { key: 'ArrowRight' })
    expect(w.text()).toContain('2 / 3')
    await w.find('[data-testid="meme-option"]').trigger('keydown', { key: 'ArrowLeft' })
    expect(w.text()).toContain('1 / 3')
  })

  it('swipes left to the next meme without emitting select', async () => {
    const w = mountSlider()
    const slide = w.find('[data-testid="meme-option"]')
    await slide.trigger('touchstart', { touches: [{ clientX: 300, clientY: 100 }] })
    await slide.trigger('touchend', { changedTouches: [{ clientX: 100, clientY: 110 }] })
    // Simulate the synthetic click a browser fires right after a swipe: it must
    // not select — the swipe flag suppresses exactly that one click.
    await slide.trigger('click')
    expect(w.text()).toContain('2 / 3')
    expect(w.emitted('select')).toBeUndefined()
  })

  it('re-tap after a swipe selects the slide (deliberate re-tap works)', async () => {
    const w = mountSlider()
    const slide = w.find('[data-testid="meme-option"]')
    // Swipe left: navigates; the synthetic click right after it must not select.
    await slide.trigger('touchstart', { touches: [{ clientX: 300, clientY: 100 }] })
    await slide.trigger('touchend', { changedTouches: [{ clientX: 100, clientY: 110 }] })
    await slide.trigger('click')
    expect(w.text()).toContain('2 / 3')
    expect(w.emitted('select')).toBeUndefined()
    // A fresh tap (touchstart/touchend without movement) followed by its click
    // MUST select: the swipe flag resets on touchstart, so a deliberate re-tap
    // right after a swipe always works.
    await slide.trigger('touchstart', { touches: [{ clientX: 150, clientY: 100 }] })
    await slide.trigger('touchend', { changedTouches: [{ clientX: 150, clientY: 100 }] })
    await slide.trigger('click')
    expect(w.emitted('select')).toEqual([['m2']])
  })

  it('shows a non-clickable placeholder when there are no memes', () => {
    const w = mountSlider({ memes: [] })
    expect(w.text()).toContain('Нет мемов')
    expect(w.find('[data-testid="meme-option"]').exists()).toBe(false)
  })
})