import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/host' },
    {
      path: '/host',
      name: 'host',
      component: () => import('@/views/host/HostView.vue'),
    },
    {
      path: '/screen',
      name: 'screen',
      component: () => import('@/views/screen/ScreenView.vue'),
    },
    {
      path: '/screen/:roomCode',
      name: 'screen-room',
      component: () => import('@/views/screen/ScreenView.vue'),
    },
    {
      path: '/play/:roomCode',
      name: 'player',
      component: () => import('@/views/player/PlayerView.vue'),
    },
  ],
})

export default router