import type {
  GetCliproxyAuthFileBindingsParams,
  GetCliproxyAuthFileBindingsResponse,
} from '../../cliproxy-auth-files/types'

const xaiQuotaBindingsPageSize = 100
const xaiQuotaBindingsMaxConcurrentRequests = 4

type GetXAIQuotaBindings = (
  params: GetCliproxyAuthFileBindingsParams
) => Promise<GetCliproxyAuthFileBindingsResponse>

export async function fetchAllXAIQuotaBindings(
  getBindings: GetXAIQuotaBindings
): Promise<GetCliproxyAuthFileBindingsResponse> {
  const firstResponse = await getBindings({
    p: 1,
    page_size: xaiQuotaBindingsPageSize,
    type: 'xai',
  })
  if (!firstResponse.success || !firstResponse.data) {
    return firstResponse
  }

  const totalPages = Math.ceil(
    firstResponse.data.total / xaiQuotaBindingsPageSize
  )
  if (totalPages <= 1) {
    return firstResponse
  }

  const remainingPageCount = totalPages - 1
  const remainingResponses: GetCliproxyAuthFileBindingsResponse[] = []
  let nextPageIndex = 0
  await Promise.all(
    Array.from(
      {
        length: Math.min(
          xaiQuotaBindingsMaxConcurrentRequests,
          remainingPageCount
        ),
      },
      async () => {
        while (nextPageIndex < remainingPageCount) {
          const pageIndex = nextPageIndex
          nextPageIndex += 1
          remainingResponses[pageIndex] = await getBindings({
            p: pageIndex + 2,
            page_size: xaiQuotaBindingsPageSize,
            type: 'xai',
          })
        }
      }
    )
  )
  const failedResponse = remainingResponses.find(
    (response) => !response.success || !response.data
  )
  if (failedResponse) {
    return failedResponse
  }

  return {
    ...firstResponse,
    data: {
      ...firstResponse.data,
      items: [
        ...firstResponse.data.items,
        ...remainingResponses.flatMap((response) => response.data?.items ?? []),
      ],
    },
  }
}
