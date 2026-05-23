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

import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Input, Table, Typography } from '@douyinfe/semi-ui';
import { API, isAdmin, showError } from '../../helpers';

const { Text } = Typography;

const daySeconds = 24 * 60 * 60;

const getDefaultStartTimestamp = () => Math.floor(Date.now() / 1000) - 30 * daySeconds;

const getDefaultEndTimestamp = () => Math.floor(Date.now() / 1000);

const formatDatetimeInput = (timestamp) => {
  const date = new Date(timestamp * 1000);
  const timezoneOffset = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - timezoneOffset).toISOString().slice(0, 16);
};

const parseTimestampFromInput = (value) => {
  if (!value) return undefined;
  const timestamp = Math.floor(new Date(value).getTime() / 1000);
  return Number.isNaN(timestamp) ? undefined : timestamp;
};

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const formatTokens = (value) => Number(value || 0).toLocaleString();

export default function UserConsumption() {
  const { t } = useTranslation();
  const [startInput, setStartInput] = useState(() =>
    formatDatetimeInput(getDefaultStartTimestamp()),
  );
  const [endInput, setEndInput] = useState(() =>
    formatDatetimeInput(getDefaultEndTimestamp()),
  );
  const [username, setUsername] = useState('');
  const [userOptions, setUserOptions] = useState([]);
  const [tokenName, setTokenName] = useState('');
  const [channelId, setChannelId] = useState('');
  const [modelName, setModelName] = useState('');
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const isAdminUser = isAdmin();

  const searchUsers = async (keyword) => {
    if (!keyword) return;
    try {
      const res = await API.get('/api/user/search', {
        params: { keyword, group: '', p: 1, page_size: 20 },
      });
      if (res.data.success) {
        setUserOptions(
          (res.data.data?.items || []).map((user) => ({
            label: `${user.username || '-'} (ID: ${user.id})`,
            value: user.username || '',
          })),
        );
      } else {
        showError(res.data.message || t('搜索用户失败'));
      }
    } catch (error) {
      showError(error);
    }
  };

  const loadConsumption = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/cliproxy/user-consumption', {
        params: {
          p: 1,
          page_size: 100,
          start_timestamp: parseTimestampFromInput(startInput),
          end_timestamp: parseTimestampFromInput(endInput),
          username: isAdminUser ? username.trim() || undefined : undefined,
          token_name: tokenName.trim() || undefined,
          channel_id: channelId.trim() || undefined,
          model_name: modelName.trim() || undefined,
        },
      });
      if (res.data.success) {
        setRows(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('Load user consumption failed'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConsumption();
  }, []);

  const totals = useMemo(
    () =>
      rows.reduce(
        (acc, row) => ({
          promptTokens: acc.promptTokens + Number(row.prompt_tokens || 0),
          completionTokens:
            acc.completionTokens + Number(row.completion_tokens || 0),
          totalTokens: acc.totalTokens + Number(row.total_tokens || 0),
        }),
        {
          promptTokens: 0,
          completionTokens: 0,
          totalTokens: 0,
        },
      ),
    [rows],
  );

  const columns = [
    {
      title: t('用户'),
      render: (_, record) => (
        <div>
          <div>{record.username || '-'}</div>
          <Text type='tertiary'>ID: {record.user_id}</Text>
        </div>
      ),
    },
    {
      title: t('Auth File / Key'),
      render: (_, record) => (
        <div>
          <div>{record.auth_name || record.auth_index || record.token_name || '-'}</div>
          <Text type='tertiary'>ID: {record.token_id}</Text>
        </div>
      ),
    },
    {
      title: t('渠道'),
      render: (_, record) => (
        <div>
          <div>{record.channel_name || '-'}</div>
          <Text type='tertiary'>ID: {record.channel_id || '-'}</Text>
        </div>
      ),
    },
    {
      title: t('模型'),
      render: (_, record) => record.model_name || '-',
    },
    {
      title: t('提示 Tokens'),
      render: (_, record) => formatTokens(record.prompt_tokens),
    },
    {
      title: t('补全 Tokens'),
      render: (_, record) => formatTokens(record.completion_tokens),
    },
    {
      title: t('总 Tokens'),
      render: (_, record) => formatTokens(record.total_tokens),
    },
    {
      title: t('最近调用'),
      render: (_, record) => formatTime(record.last_called_at),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='space-y-4'>
        <Card title={t('Consumption Filters')}>
          <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-6'>
            <Input
              type='datetime-local'
              value={startInput}
              onChange={setStartInput}
            />
            <Input type='datetime-local' value={endInput} onChange={setEndInput} />
            {isAdminUser && (
              <Form.Select
                value={username}
                placeholder={t('搜索并选择用户')}
                filter
                remote
                optionList={userOptions}
                onSearch={searchUsers}
                onChange={setUsername}
              />
            )}
            <Input
              value={tokenName}
              placeholder={t('Filter by token name')}
              onChange={setTokenName}
            />
            <Input
              value={channelId}
              placeholder={t('Filter by channel ID')}
              onChange={setChannelId}
            />
            <Input
              value={modelName}
              placeholder={t('Filter by model name')}
              onChange={setModelName}
            />
            <Button type='primary' loading={loading} onClick={loadConsumption}>
              {t('查询')}
            </Button>
          </div>
        </Card>

        <div className='grid gap-4 md:grid-cols-3'>
          <Card title={t('提示 Tokens')}>{formatTokens(totals.promptTokens)}</Card>
          <Card title={t('补全 Tokens')}>
            {formatTokens(totals.completionTokens)}
          </Card>
          <Card title={t('总 Tokens')}>{formatTokens(totals.totalTokens)}</Card>
        </div>

        <Card title={t('User Consumption Details')}>
          <Table
            columns={columns}
            dataSource={rows}
            loading={loading}
            pagination={false}
            rowKey={(record) =>
              `${record.user_id}-${record.token_id}-${record.channel_id}-${record.model_name}`
            }
          />
        </Card>
      </div>
    </div>
  );
}
