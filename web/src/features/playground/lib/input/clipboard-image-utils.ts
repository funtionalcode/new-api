/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
type ClipboardImageItem = {
  types: readonly string[]
  getType: (type: string) => Promise<Blob>
}

type ReadClipboardItems = () => Promise<ClipboardImageItem[]>

function defaultReadClipboardItems(): Promise<ClipboardImageItem[]> {
  if (typeof navigator === 'undefined' || !navigator.clipboard?.read) {
    return Promise.resolve([])
  }

  return navigator.clipboard.read()
}

function imageExtensionFromType(type: string): string {
  const subtype = type.split('/')[1]?.split('+')[0]?.trim()
  return subtype || 'png'
}

export async function readClipboardImageFiles(
  readClipboardItems: ReadClipboardItems = defaultReadClipboardItems
): Promise<File[]> {
  const items = await readClipboardItems()
  const files: File[] = []

  for (const item of items) {
    const imageType = item.types.find((type) => type.startsWith('image/'))
    if (!imageType) {
      continue
    }

    const blob = await item.getType(imageType)
    const extension = imageExtensionFromType(blob.type || imageType)
    files.push(
      new File([blob], `screenshot-${Date.now()}.${extension}`, {
        type: blob.type || imageType,
      })
    )
  }

  return files
}
