import { createFileRoute, redirect } from '@tanstack/react-router'

import { ModelStatus } from '@/features/model-status'
import { getFreshModuleAccess } from '@/lib/nav-modules'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/model-status/')({
  beforeLoad: async ({ location }) => {
    const access = await getFreshModuleAccess('modelStatus')
    if (!access.enabled) {
      throw redirect({ to: '/' })
    }
    if (access.requireAuth) {
      const { auth } = useAuthStore.getState()
      if (!auth.user) {
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    }
  },
  component: ModelStatus,
})
