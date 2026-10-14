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
import { Maximize, Minimize, Square } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  WebPreview,
  WebPreviewBody,
  WebPreviewNavigation,
} from '@/components/ai-elements/web-preview'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

export function PelicanPreview(props: {
  document: string
  onStop: () => void
}) {
  const { t } = useTranslation()
  const previewRef = useRef<HTMLDivElement>(null)
  const [fullscreen, setFullscreen] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)

  useEffect(() => {
    const syncFullscreen = () =>
      setFullscreen(document.fullscreenElement === previewRef.current)
    document.addEventListener('fullscreenchange', syncFullscreen)
    return () =>
      document.removeEventListener('fullscreenchange', syncFullscreen)
  }, [])

  const toggleFullscreen = async () => {
    const element = previewRef.current
    if (!element) return
    try {
      if (document.fullscreenElement === element) {
        await document.exitFullscreen()
      } else if (element.requestFullscreen) {
        await element.requestFullscreen()
      } else {
        setDialogOpen(true)
      }
    } catch {
      // 浏览器不允许原生全屏时，复用大尺寸弹窗；不同时运行两份生成代码。
      setDialogOpen(true)
    }
  }

  const stopPreview = async () => {
    if (document.fullscreenElement === previewRef.current) {
      try {
        await document.exitFullscreen()
      } catch {
        // 移除预览节点后浏览器也会退出该节点的全屏，不阻止用户停止动画。
      }
    }
    setDialogOpen(false)
    props.onStop()
  }

  return (
    <>
      <WebPreview
        ref={previewRef}
        role='region'
        aria-label={t('Pelican HTML preview')}
        className='h-[780px] overflow-hidden bg-white [&:fullscreen]:h-dvh [&:fullscreen]:w-screen [&:fullscreen]:rounded-none'
      >
        <WebPreviewNavigation className='bg-background shrink-0 flex-wrap justify-end gap-2'>
          <span className='text-muted-foreground mr-auto text-xs'>
            {t('Preview')}: 1280 × 720 px
          </span>
          <Button size='sm' variant='outline' onClick={toggleFullscreen}>
            {fullscreen ? (
              <Minimize aria-hidden='true' />
            ) : (
              <Maximize aria-hidden='true' />
            )}
            {fullscreen ? t('Exit fullscreen') : t('Fullscreen preview')}
          </Button>
          <Button size='sm' variant='ghost' onClick={stopPreview}>
            <Square aria-hidden='true' />
            {t('Stop preview')}
          </Button>
        </WebPreviewNavigation>
        {!dialogOpen && <PelicanPreviewViewport document={props.document} />}
      </WebPreview>
      <Dialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={t('Pelican HTML preview')}
        showCloseButton={false}
        contentClassName='h-[calc(100dvh-2rem)] max-w-[calc(100vw-2rem)] gap-2 p-3 sm:max-w-[calc(100vw-2rem)] sm:p-3'
        contentHeight='calc(100dvh - 9rem)'
        bodyClassName='h-full py-0'
        footerClassName='sm:mx-0 sm:mb-0 sm:p-0'
        footer={
          <>
            <Button
              size='sm'
              variant='outline'
              onClick={() => setDialogOpen(false)}
            >
              <Minimize aria-hidden='true' />
              {t('Exit fullscreen')}
            </Button>
            <Button size='sm' variant='ghost' onClick={stopPreview}>
              <Square aria-hidden='true' />
              {t('Stop preview')}
            </Button>
          </>
        }
      >
        {dialogOpen && (
          <WebPreview className='h-full min-h-0 overflow-hidden bg-white'>
            <PelicanPreviewViewport document={props.document} />
          </WebPreview>
        )}
      </Dialog>
    </>
  )
}

function PelicanPreviewViewport(props: { document: string }) {
  const { t } = useTranslation()
  return (
    <div
      role='region'
      aria-label={t('Preview')}
      className='min-h-0 flex-1 overflow-auto'
    >
      <div className='mx-auto h-[720px] w-[1280px]'>
        <WebPreviewBody
          srcDoc={props.document}
          sandbox='allow-scripts'
          referrerPolicy='no-referrer'
          title={t('Pelican HTML preview')}
          className='block h-[720px] w-[1280px] max-w-none'
        />
      </div>
    </div>
  )
}
