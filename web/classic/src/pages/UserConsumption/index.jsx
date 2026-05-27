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
import { VChart } from '@visactor/react-vchart';
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

const formatQuota = (value) => {
  const num = Number(value || 0);
  if (num >= 1e8) return `${(num / 1e8).toFixed(2)}亿`;
  if (num >= 1e4) return `${(num / 1e4).toFixed(2)}万`;
  return num.toLocaleString();
};

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
  const [authIndex, setAuthIndex] = useState('');
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('trend');
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
          auth_index: authIndex.trim() || undefined,
        },
      });
      if (res.data.success) {
        setRows(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('加载用户消耗数据失败'));
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

  const stats = useMemo(() => {
    if (!rows || rows.length === 0) {
      return { totalTokens: 0, totalCount: 0, avgTokens: 0, totalQuota: 0 };
    }

    let totalTokens = 0;
    let totalCount = 0;
    let totalQuota = 0;

    for (const item of rows) {
      totalTokens += Number(item.total_tokens || 0);
      totalCount += Number(item.request_count || 0);
      totalQuota += Number(item.quota || 0);
    }

    return {
      totalTokens,
      totalCount,
      avgTokens: totalCount > 0 ? Math.round(totalTokens / totalCount) : 0,
      totalQuota,
    };
  }, [rows]);

  const tokenGroupedData = useMemo(() => {
    const tokenMap = new Map();

    for (const row of rows) {
      const key = row.token_id;
      const existing = tokenMap.get(key) || {
        token_id: row.token_id,
        token_name: row.token_name,
        auth_index: row.auth_index,
        auth_name: row.auth_name,
        total_tokens: 0,
        prompt_tokens: 0,
        completion_tokens: 0,
        request_count: 0,
        quota: 0,
        users: new Set(),
        last_called_at: 0,
      };

      existing.total_tokens += Number(row.total_tokens || 0);
      existing.prompt_tokens += Number(row.prompt_tokens || 0);
      existing.completion_tokens += Number(row.completion_tokens || 0);
      existing.request_count += Number(row.request_count || 0);
      existing.quota += Number(row.quota || 0);
      if (row.username) existing.users.add(row.username);
      existing.last_called_at = Math.max(existing.last_called_at, row.last_called_at || 0);

      tokenMap.set(key, existing);
    }

    return Array.from(tokenMap.values()).sort((a, b) => b.total_tokens - a.total_tokens);
  }, [rows]);

  const chartData = useMemo(() => {
    const topN = 15;
    const topItems = tokenGroupedData.slice(0, topN);
    const otherTokens = tokenGroupedData.slice(topN).reduce((sum, item) => sum + item.total_tokens, 0);
    return otherTokens > 0
      ? [...topItems, { token_name: t('其他'), total_tokens: otherTokens }]
      : topItems;
  }, [tokenGroupedData, t]);

  const trendSpec = useMemo(() => ({
    type: 'area',
    data: [{
      values: chartData.map((item) => ({
        x: item.token_name || `Token #${item.token_id}`,
        y: item.total_tokens,
      })),
    }],
    xField: 'x',
    yField: 'y',
    color: ['var(--semi-color-primary)'],
    area: { visible: true, opacity: 0.3 },
    line: { visible: true, style: { lineWidth: 2 } },
    point: { visible: false },
    axes: [
      { orient: 'bottom', label: { visible: true, style: { angle: -45, textAlign: 'right' } } },
      { orient: 'left', label: { visible: true, formatMethod: (val) => val.toLocaleString() } },
    ],
    tooltip: {
      mark: {
        content: [{ key: t('Tokens'), value: (datum) => datum.y?.toLocaleString() }],
      },
    },
  }), [chartData, t]);

  const proportionSpec = useMemo(() => ({
    type: 'pie',
    data: [{
      values: chartData.map((item) => ({
        type: item.token_name || `Token #${item.token_id}`,
        value: item.total_tokens,
      })),
    }],
    valueField: 'value',
    categoryField: 'type',
    label: { visible: true, position: 'outside', formatMethod: (val) => val.toLocaleString() },
    tooltip: {
      mark: {
        content: [{ key: t('Tokens'), value: (datum) => datum.value?.toLocaleString() }],
      },
    },
  }), [chartData, t]);

  const topSpec = useMemo(() => ({
    type: 'bar',
    data: [{
      values: chartData.map((item) => ({
        x: item.token_name || `Token #${item.token_id}`,
        y: item.total_tokens,
      })),
    }],
    xField: 'x',
    yField: 'y',
    color: ['var(--semi-color-secondary)'],
    bar: { style: { cornerRadius: [4, 4, 0, 0] } },
    axes: [
      { orient: 'bottom', label: { visible: true, style: { angle: -45, textAlign: 'right' } } },
      { orient: 'left', label: { visible: true, formatMethod: (val) => val.toLocaleString() } },
    ],
    tooltip: {
      mark: {
        content: [{ key: t('Tokens'), value: (datum) => datum.y?.toLocaleString() }],
      },
    },
  }), [chartData, t]);

  const getChartSpec = () => {
    switch (activeTab) {
      case 'proportion': return proportionSpec;
      case 'top': return topSpec;
      default: return trendSpec;
    }
  };

  const statCards = [
    { title: t('总 Tokens'), value: formatTokens(stats.totalTokens), desc: t('所有消耗的 Tokens') },
    { title: t('总请求数'), value: formatTokens(stats.totalCount), desc: t('API 调用次数') },
    { title: t('平均 Tokens'), value: formatTokens(stats.avgTokens), desc: t('每次请求平均') },
    { title: t('总配额'), value: formatQuota(stats.totalQuota), desc: t('消耗的配额') },
  ];

  const columns = [
    {
      title: t('Auth File / Key'),
      render: (_, record) => (
        <div>
          <div>{record.token_name || '-'}</div>
          <Text type='tertiary'>ID: {record.token_id}</Text>
        </div>
      ),
    },
    {
      title: t('用户'),
      render: (_, record) => (
        <div>
          <div>{record.users.size}</div>
          <Text type='tertiary'>
            {Array.from(record.users).slice(0, 3).join(', ')}
            {record.users.size > 3 && ` +${record.users.size - 3}`}
          </Text>
        </div>
      ),
    },
    {
      title: t('请求数'),
      render: (_, record) => formatTokens(record.request_count),
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
      title: t('配额'),
      render: (_, record) => formatQuota(record.quota),
    },
    {
      title: t('最近调用'),
      render: (_, record) => formatTime(record.last_called_at),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='space-y-4'>
        <Card title={t('消耗过滤')}>
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
              placeholder={t('按令牌名称过滤')}
              onChange={setTokenName}
            />
            <Input
              value={authIndex}
              placeholder={t('按认证文件过滤')}
              onChange={setAuthIndex}
            />
            <Button type='primary' loading={loading} onClick={loadConsumption}>
              {t('查询')}
            </Button>
          </div>
        </Card>

        <div className='grid gap-4 md:grid-cols-4'>
          {statCards.map((card) => (
            <Card key={card.title}>
              <div className='text-xs text-gray-500 mb-1'>{card.title}</div>
              <div className='text-2xl font-bold'>{card.value}</div>
              <div className='text-xs text-gray-400 mt-1'>{card.desc}</div>
            </Card>
          ))}
        </div>

        <Card
          title={t('Token 消耗')}
          headerExtraContent={
            <div className='flex gap-2'>
              {[
                { key: 'trend', label: t('趋势') },
                { key: 'proportion', label: t('分布') },
                { key: 'top', label: t('排行') },
              ].map((tab) => (
                <Button
                  key={tab.key}
                  size='small'
                  type={activeTab === tab.key ? 'primary' : 'tertiary'}
                  onClick={() => setActiveTab(tab.key)}
                >
                  {tab.label}
                </Button>
              ))}
            </div>
          }
        >
          <div className='h-[300px]'>
            {chartData.length > 0 ? (
              <VChart
                key={`token-chart-${activeTab}`}
                spec={getChartSpec()}
                option={{ theme: { isHorizontal: false } }}
              />
            ) : (
              <div className='flex h-full items-center justify-center text-gray-500'>
                {t('暂无消耗数据')}
              </div>
            )}
          </div>
        </Card>

        <Card title={t('Token 消耗详情')}>
          <Table
            columns={columns}
            dataSource={tokenGroupedData}
            loading={loading}
            pagination={false}
            rowKey='token_id'
          />
        </Card>
      </div>
    </div>
  );
}
