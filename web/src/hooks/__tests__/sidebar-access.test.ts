import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { NavItem } from '../../components/layout/types'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
} from '../../lib/admin-permissions'
import { ROLE } from '../../lib/roles'
import type { AuthUser } from '../../stores/auth-store'
import { canShowNavItemByAccess } from '../lib/sidebar-access'

const channelItem: NavItem = {
  title: 'Channels',
  url: '/channels',
  requiredPermission: {
    resource: ADMIN_PERMISSION_RESOURCES.CHANNEL,
    action: ADMIN_PERMISSION_ACTIONS.READ,
  },
}

function userWithChannelRead(allowed: boolean): AuthUser {
  return {
    id: 22,
    username: 'magic_long',
    role: ROLE.ADMIN,
    permissions: {
      admin_permissions: {
        [ADMIN_PERMISSION_RESOURCES.CHANNEL]: {
          [ADMIN_PERMISSION_ACTIONS.READ]: allowed,
        },
      },
    },
  }
}

describe('sidebar access filtering', () => {
  test('hides channel menu item when channel read permission is denied', () => {
    assert.equal(
      canShowNavItemByAccess(channelItem, userWithChannelRead(false)),
      false
    )
  })

  test('shows channel menu item when channel read permission is allowed', () => {
    assert.equal(
      canShowNavItemByAccess(channelItem, userWithChannelRead(true)),
      true
    )
  })

  test('keeps root-only items hidden from regular admins', () => {
    const systemInfoItem: NavItem = {
      title: 'System Info',
      url: '/system-info',
      requiredRole: ROLE.SUPER_ADMIN,
    }

    assert.equal(
      canShowNavItemByAccess(systemInfoItem, userWithChannelRead(true)),
      false
    )
  })
})
