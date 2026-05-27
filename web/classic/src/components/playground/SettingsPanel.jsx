/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';
import {
  Card,
  Select,
  Typography,
  Button,
  Switch,
  InputNumber,
} from '@douyinfe/semi-ui';
import {
  Sparkles,
  Users,
  ToggleLeft,
  X,
  Settings,
  Volume2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { renderGroupOption, selectFilter } from '../../helpers';
import ParameterControl from './ParameterControl';
import ImageUrlInput from './ImageUrlInput';
import ConfigManager from './ConfigManager';
import CustomRequestEditor from './CustomRequestEditor';

const TTS_VOICE_OPTIONS = [
  { label: 'alloy', value: 'alloy' },
  { label: 'ash', value: 'ash' },
  { label: 'ballad', value: 'ballad' },
  { label: 'coral', value: 'coral' },
  { label: 'echo', value: 'echo' },
  { label: 'fable', value: 'fable' },
  { label: 'nova', value: 'nova' },
  { label: 'onyx', value: 'onyx' },
  { label: 'sage', value: 'sage' },
  { label: 'shimmer', value: 'shimmer' },
];

const TTS_FORMAT_OPTIONS = [
  { label: 'mp3', value: 'mp3' },
  { label: 'opus', value: 'opus' },
  { label: 'aac', value: 'aac' },
  { label: 'flac', value: 'flac' },
  { label: 'wav', value: 'wav' },
  { label: 'pcm', value: 'pcm' },
];

const SettingsPanel = ({
  inputs,
  parameterEnabled,
  models,
  groups,
  styleState,
  showDebugPanel,
  customRequestMode,
  customRequestBody,
  onInputChange,
  onParameterToggle,
  onCloseSettings,
  onConfigImport,
  onConfigReset,
  onCustomRequestModeChange,
  onCustomRequestBodyChange,
  previewPayload,
  messages,
}) => {
  const { t } = useTranslation();

  const currentConfig = {
    inputs,
    parameterEnabled,
    showDebugPanel,
    customRequestMode,
    customRequestBody,
  };

  return (
    <Card
      className='h-full flex flex-col'
      bordered={false}
      bodyStyle={{
        padding: styleState.isMobile ? '16px' : '24px',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* 标题区域 - 与调试面板保持一致 */}
      <div className='flex items-center justify-between mb-6 flex-shrink-0'>
        <div className='flex items-center'>
          <div className='w-10 h-10 rounded-full bg-gradient-to-r from-purple-500 to-pink-500 flex items-center justify-center mr-3'>
            <Settings size={20} className='text-white' />
          </div>
          <Typography.Title heading={5} className='mb-0'>
            {t('模型配置')}
          </Typography.Title>
        </div>

        {styleState.isMobile && onCloseSettings && (
          <Button
            icon={<X size={16} />}
            onClick={onCloseSettings}
            theme='borderless'
            type='tertiary'
            size='small'
            className='!rounded-lg'
          />
        )}
      </div>

      {/* 移动端配置管理 */}
      {styleState.isMobile && (
        <div className='mb-4 flex-shrink-0'>
          <ConfigManager
            currentConfig={currentConfig}
            onConfigImport={onConfigImport}
            onConfigReset={onConfigReset}
            styleState={{ ...styleState, isMobile: false }}
            messages={messages}
          />
        </div>
      )}

      <div className='space-y-6 overflow-y-auto flex-1 pr-2 model-settings-scroll'>
        {/* 自定义请求体编辑器 */}
        <CustomRequestEditor
          customRequestMode={customRequestMode}
          customRequestBody={customRequestBody}
          onCustomRequestModeChange={onCustomRequestModeChange}
          onCustomRequestBodyChange={onCustomRequestBodyChange}
          defaultPayload={previewPayload}
        />

        {/* 分组选择 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <div className='flex items-center gap-2 mb-2'>
            <Users size={16} className='text-gray-500' />
            <Typography.Text strong className='text-sm'>
              {t('分组')}
            </Typography.Text>
            {customRequestMode && (
              <Typography.Text className='text-xs text-orange-600'>
                ({t('已在自定义模式中忽略')})
              </Typography.Text>
            )}
          </div>
          <Select
            placeholder={t('请选择分组')}
            name='group'
            required
            selection
            filter={selectFilter}
            autoClearSearchValue={false}
            onChange={(value) => onInputChange('group', value)}
            value={inputs.group}
            autoComplete='new-password'
            optionList={groups}
            renderOptionItem={renderGroupOption}
            style={{ width: '100%' }}
            dropdownStyle={{ width: '100%', maxWidth: '100%' }}
            className='!rounded-lg'
            disabled={customRequestMode}
          />
        </div>

        {/* 模型选择 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <div className='flex items-center gap-2 mb-2'>
            <Sparkles size={16} className='text-gray-500' />
            <Typography.Text strong className='text-sm'>
              {t('模型')}
            </Typography.Text>
            {customRequestMode && (
              <Typography.Text className='text-xs text-orange-600'>
                ({t('已在自定义模式中忽略')})
              </Typography.Text>
            )}
          </div>
          <Select
            placeholder={t('请选择模型')}
            name='model'
            required
            selection
            filter={selectFilter}
            autoClearSearchValue={false}
            onChange={(value) => onInputChange('model', value)}
            value={inputs.model}
            autoComplete='new-password'
            optionList={models}
            style={{ width: '100%' }}
            dropdownStyle={{ width: '100%', maxWidth: '100%' }}
            className='!rounded-lg'
            disabled={customRequestMode}
          />
        </div>

        {/* 图片URL输入 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <ImageUrlInput
            imageUrls={inputs.imageUrls}
            imageEnabled={inputs.imageEnabled}
            onImageUrlsChange={(urls) => onInputChange('imageUrls', urls)}
            onImageEnabledChange={(enabled) =>
              onInputChange('imageEnabled', enabled)
            }
            disabled={customRequestMode}
          />
        </div>

        {/* TTS 模式 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <div className='flex items-center justify-between mb-3'>
            <div className='flex items-center gap-2'>
              <Volume2 size={16} className='text-gray-500' />
              <Typography.Text strong className='text-sm'>
                {t('TTS 语音输出')}
              </Typography.Text>
              {customRequestMode && (
                <Typography.Text className='text-xs text-orange-600'>
                  ({t('已在自定义模式中忽略')})
                </Typography.Text>
              )}
            </div>
            <Switch
              checked={!!inputs.ttsEnabled}
              onChange={(checked) => onInputChange('ttsEnabled', checked)}
              checkedText={t('开')}
              uncheckedText={t('关')}
              size='small'
              disabled={customRequestMode}
            />
          </div>
          <Typography.Text className='text-xs text-gray-500 block mb-3'>
            {t('启用后将输入文本转换为可播放语音')}
          </Typography.Text>
          {inputs.ttsEnabled && (
            <div className='space-y-3'>
              <div>
                <Typography.Text className='text-xs text-gray-500 block mb-1'>
                  {t('音色')}
                </Typography.Text>
                <Select
                  value={inputs.ttsVoice || 'alloy'}
                  onChange={(value) => onInputChange('ttsVoice', value)}
                  optionList={TTS_VOICE_OPTIONS}
                  style={{ width: '100%' }}
                  className='!rounded-lg'
                  disabled={customRequestMode}
                />
              </div>
              <div>
                <Typography.Text className='text-xs text-gray-500 block mb-1'>
                  {t('音频格式')}
                </Typography.Text>
                <Select
                  value={inputs.ttsResponseFormat || 'mp3'}
                  onChange={(value) =>
                    onInputChange('ttsResponseFormat', value)
                  }
                  optionList={TTS_FORMAT_OPTIONS}
                  style={{ width: '100%' }}
                  className='!rounded-lg'
                  disabled={customRequestMode}
                />
              </div>
              <div>
                <Typography.Text className='text-xs text-gray-500 block mb-1'>
                  {t('语速')}
                </Typography.Text>
                <InputNumber
                  value={inputs.ttsSpeed ?? 1}
                  onNumberChange={(value) => onInputChange('ttsSpeed', value)}
                  min={0.25}
                  max={4}
                  step={0.05}
                  precision={2}
                  style={{ width: '100%' }}
                  disabled={customRequestMode}
                />
              </div>
            </div>
          )}
        </div>

        {/* 参数控制组件 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <ParameterControl
            inputs={inputs}
            parameterEnabled={parameterEnabled}
            onInputChange={onInputChange}
            onParameterToggle={onParameterToggle}
            disabled={customRequestMode}
          />
        </div>

        {/* 流式输出开关 */}
        <div className={customRequestMode ? 'opacity-50' : ''}>
          <div className='flex items-center justify-between'>
            <div className='flex items-center gap-2'>
              <ToggleLeft size={16} className='text-gray-500' />
              <Typography.Text strong className='text-sm'>
                {t('流式输出')}
              </Typography.Text>
              {customRequestMode && (
                <Typography.Text className='text-xs text-orange-600'>
                  ({t('已在自定义模式中忽略')})
                </Typography.Text>
              )}
            </div>
            <Switch
              checked={inputs.stream}
              onChange={(checked) => onInputChange('stream', checked)}
              checkedText={t('开')}
              uncheckedText={t('关')}
              size='small'
              disabled={customRequestMode}
            />
          </div>
        </div>
      </div>

      {/* 桌面端的配置管理放在底部 */}
      {!styleState.isMobile && (
        <div className='flex-shrink-0 pt-3'>
          <ConfigManager
            currentConfig={currentConfig}
            onConfigImport={onConfigImport}
            onConfigReset={onConfigReset}
            styleState={styleState}
            messages={messages}
          />
        </div>
      )}
    </Card>
  );
};

export default SettingsPanel;
