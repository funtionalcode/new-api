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
import type { VideoGenerationResponse } from '../../types'

type PollVideoGenerationOptions = {
  videoId: string
  initialVideo: VideoGenerationResponse
  signal: AbortSignal
  failureMessage: string
  getVideoGeneration: (
    id: string,
    signal?: AbortSignal
  ) => Promise<VideoGenerationResponse>
  waitForPollInterval: (signal: AbortSignal) => Promise<void>
}

export async function pollVideoGeneration({
  videoId,
  initialVideo,
  signal,
  failureMessage,
  getVideoGeneration,
  waitForPollInterval,
}: PollVideoGenerationOptions): Promise<VideoGenerationResponse> {
  let video = initialVideo

  while (true) {
    throwIfAborted(signal)

    const status = video.status.toLowerCase()
    if (status === 'completed') {
      return video
    }
    if (status === 'failed') {
      throw new Error(video.error?.message || failureMessage)
    }

    await waitForPollInterval(signal)
    video = await getVideoGeneration(videoId, signal)
  }
}

function throwIfAborted(signal: AbortSignal): void {
  if (signal.aborted) {
    throw signal.reason ?? new Error('Aborted')
  }
}
