import assert from 'node:assert/strict'
import { afterEach, describe, test } from 'node:test'

import { isPasskeySupported } from './passkey'

const originalWindow = globalThis.window

function setTestWindow(value: unknown) {
  Object.defineProperty(globalThis, 'window', {
    value,
    configurable: true,
  })
}

afterEach(() => {
  Object.defineProperty(globalThis, 'window', {
    value: originalWindow,
    configurable: true,
  })
})

describe('passkey support detection', () => {
  test('allows WebAuthn when only a roaming authenticator may be available', async () => {
    setTestWindow({
      PublicKeyCredential: {
        isConditionalMediationAvailable: async () => false,
        isUserVerifyingPlatformAuthenticatorAvailable: async () => false,
      },
    })

    assert.equal(await isPasskeySupported(), true)
  })

  test('rejects environments without WebAuthn API', async () => {
    setTestWindow({})

    assert.equal(await isPasskeySupported(), false)
  })
})
