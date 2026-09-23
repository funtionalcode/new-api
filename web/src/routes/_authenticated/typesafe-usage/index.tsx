import { createFileRoute, redirect } from '@tanstack/react-router'

import { QuotaBindingsPage } from '@/features/quota-bindings'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/typesafe-usage/')({
  beforeLoad: () => {
    if ((useAuthStore.getState().auth.user?.role ?? 0) < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: () => <QuotaBindingsPage provider='typesafe' />,
})
