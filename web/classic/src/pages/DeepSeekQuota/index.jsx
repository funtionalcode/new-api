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

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Form,
  Modal,
  Progress,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, isAdmin, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const emptyQuotaForm = {
  id: undefined,
  name: '',
  note: '',
  request_curl: '',
  enabled: true,
};

const buildQuotaForm = (binding = emptyQuotaForm) => ({
  id: binding.id,
  name: binding.name || '',
  note: binding.note || '',
  request_curl: '',
  enabled: binding.enabled !== false,
});

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const formatTokens = (value) => {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) return '0';
  return number.toLocaleString();
};

const formatAmount = (value) => {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) return String(value || '0');
  return number.toLocaleString(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  });
};

const normalizeUsagePercent = (value) => {
  const percent = Number(value || 0);
  if (!Number.isFinite(percent)) return 0;
  return Math.min(100, Math.max(0, Math.round(percent)));
};

const getUsageColor = (percent) => {
  if (percent >= 90) return 'var(--semi-color-danger)';
  if (percent >= 70) return 'var(--semi-color-warning)';
  return 'var(--semi-color-success)';
};

const parseJsonList = (value) => {
  if (Array.isArray(value)) return value;
  if (typeof value !== 'string' || !value.trim()) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
};

const renderMonthlyQuota = (record) => {
  const limit = Number(record.last_monthly_limit_tokens || 0);
  const used = Number(record.last_monthly_used_tokens || 0);
  const remaining = Number(record.last_monthly_remaining_tokens || 0);
  const percent = normalizeUsagePercent(record.last_monthly_percent);
  const hasLimit = limit > 0;
  const display = hasLimit
    ? `${formatTokens(used)} / ${formatTokens(limit)}`
    : `${formatTokens(used)} / 未获取`;

  return (
    <Tooltip content={display}>
      <div style={{ minWidth: 150 }}>
        <Progress
          percent={hasLimit ? percent : 0}
          stroke={getUsageColor(percent)}
          format={() => (hasLimit ? `${percent}%` : '-')}
          style={{ marginBottom: 2 }}
        />
        <Text type='tertiary'>{display}</Text>
        {hasLimit ? (
          <div>
            <Text type='tertiary'>剩余 {formatTokens(remaining)}</Text>
          </div>
        ) : null}
      </div>
    </Tooltip>
  );
};

const renderWallets = (normalWalletsValue, bonusWalletsValue) => {
  const normalWallets = parseJsonList(normalWalletsValue);
  const bonusWallets = parseJsonList(bonusWalletsValue);
  const wallets = [
    ...normalWallets.map((wallet) => ({ ...wallet, type: '余额' })),
    ...bonusWallets.map((wallet) => ({ ...wallet, type: '赠金' })),
  ];
  if (wallets.length === 0) return '-';

  const content = wallets
    .map(
      (wallet) =>
        `${wallet.type} ${wallet.currency || ''}: ${formatAmount(
          wallet.balance,
        )}，约 ${formatTokens(wallet.token_estimation)} tokens`,
    )
    .join('\n');
  const primary = wallets[0];
  return (
    <Tooltip content={<pre style={{ margin: 0 }}>{content}</pre>}>
      <div className='max-w-[180px] truncate'>
        {primary.currency || '-'} {formatAmount(primary.balance)}
      </div>
    </Tooltip>
  );
};

const renderMonthlyCosts = (value) => {
  const costs = parseJsonList(value);
  if (costs.length === 0) return '-';
  const content = costs
    .map((cost) => `${cost.currency || ''}: ${formatAmount(cost.amount)}`)
    .join('\n');
  const primary = costs[0];
  return (
    <Tooltip content={<pre style={{ margin: 0 }}>{content}</pre>}>
      <span>
        {primary.currency || '-'} {formatAmount(primary.amount)}
      </span>
    </Tooltip>
  );
};

export default function DeepSeekQuota() {
  const { t } = useTranslation();
  const [bindings, setBindings] = useState([]);
  const [bindingLoading, setBindingLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [quotaForm, setQuotaForm] = useState(emptyQuotaForm);
  const [refreshingBindingId, setRefreshingBindingId] = useState(undefined);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const adminUser = isAdmin();

  const loadBindings = async () => {
    setBindingLoading(true);
    try {
      const res = await API.get('/api/deepseek-quota/bindings', {
        params: { p: 1, page_size: 100 },
      });
      if (res.data.success) {
        setBindings(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('加载 DeepSeek 额度配置失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setBindingLoading(false);
    }
  };

  const openCreateModal = () => {
    setQuotaForm(emptyQuotaForm);
    setModalVisible(true);
  };

  const openEditModal = (binding) => {
    setQuotaForm(buildQuotaForm(binding));
    setModalVisible(true);
  };

  const saveBinding = async () => {
    if (!quotaForm.name.trim()) {
      showError(t('请输入名称'));
      return;
    }
    if (!quotaForm.id && !quotaForm.request_curl.trim()) {
      showError(t('请输入 DeepSeek 额度 curl'));
      return;
    }
    setSaving(true);
    try {
      const payload = {
        name: quotaForm.name.trim(),
        note: quotaForm.note.trim(),
        request_curl: quotaForm.request_curl.trim(),
        enabled: quotaForm.enabled,
      };
      const res = quotaForm.id
        ? await API.put(
            `/api/deepseek-quota/bindings/${quotaForm.id}`,
            payload,
          )
        : await API.post('/api/deepseek-quota/bindings', payload);
      if (res.data.success) {
        showSuccess(t('保存成功'));
        setModalVisible(false);
        setQuotaForm(emptyQuotaForm);
        await loadBindings();
      } else {
        showError(res.data.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const deleteBinding = (binding) => {
    Modal.confirm({
      title: t('确认删除 DeepSeek 额度配置？'),
      content: binding.name || '-',
      onOk: async () => {
        try {
          const res = await API.delete(
            `/api/deepseek-quota/bindings/${binding.id}`,
          );
          if (res.data.success) {
            showSuccess(t('删除成功'));
            await loadBindings();
          } else {
            showError(res.data.message || t('删除失败'));
          }
        } catch (error) {
          showError(error);
        }
      },
    });
  };

  const refreshUsage = async (binding) => {
    setRefreshingBindingId(binding.id);
    try {
      const res = await API.post(
        `/api/deepseek-quota/bindings/${binding.id}/refresh-usage`,
      );
      if (res.data.success) {
        const refreshed = res.data.data;
        if (refreshed?.last_error) {
          showError(refreshed.last_error);
        } else {
          showSuccess(t('刷新成功'));
        }
        await loadBindings();
      } else {
        showError(res.data.message || t('刷新失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setRefreshingBindingId(undefined);
    }
  };

  const refreshAllUsage = async () => {
    const enabledBindings = bindings.filter((binding) => binding.enabled);
    if (enabledBindings.length === 0) {
      showError(t('没有需要刷新的 DeepSeek 额度配置'));
      return;
    }
    setRefreshingAll(true);
    let successCount = 0;
    let failCount = 0;
    try {
      const results = await Promise.allSettled(
        enabledBindings.map((binding) =>
          API.post(
            `/api/deepseek-quota/bindings/${binding.id}/refresh-usage`,
          ),
        ),
      );
      results.forEach((result) => {
        const data = result.value?.data?.data;
        if (
          result.status === 'fulfilled' &&
          result.value?.data?.success &&
          !data?.last_error
        ) {
          successCount++;
        } else {
          failCount++;
        }
      });
      if (failCount === 0) {
        showSuccess(
          t('全部刷新成功，共 {{count}} 条', { count: successCount }),
        );
      } else {
        showError(
          t('刷新完成：成功 {{success}} 条，失败 {{fail}} 条', {
            success: successCount,
            fail: failCount,
          }),
        );
      }
      await loadBindings();
    } catch (error) {
      showError(error);
    } finally {
      setRefreshingAll(false);
    }
  };

  useEffect(() => {
    loadBindings();
  }, []);

  const columns = [
    {
      title: t('名称'),
      render: (_, record) => (
        <div>
          <div>{record.name || '-'}</div>
          {record.note ? (
            <Tooltip content={record.note}>
              <Text type='tertiary' className='max-w-[180px] truncate block'>
                {record.note}
              </Text>
            </Tooltip>
          ) : null}
        </div>
      ),
    },
    {
      title: t('本月额度'),
      render: (_, record) => renderMonthlyQuota(record),
    },
    {
      title: t('可用 Token 估算'),
      render: (_, record) =>
        formatTokens(record.last_total_available_tokens),
    },
    {
      title: t('钱包余额'),
      render: (_, record) =>
        renderWallets(record.last_normal_wallets, record.last_bonus_wallets),
    },
    {
      title: t('本月消费'),
      render: (_, record) => renderMonthlyCosts(record.last_monthly_costs),
    },
    {
      title: t('刷新进度'),
      render: (_, record) => {
        if (!record.last_refreshed_at) {
          return (
            <div>
              <div>{t('未刷新')}</div>
              <Text type='tertiary'>-</Text>
            </div>
          );
        }
        if (record.last_error) {
          return (
            <Tooltip content={record.last_error}>
              <div>
                <Text type='danger'>{t('刷新失败')}</Text>
                <div>
                  <Text type='tertiary'>
                    {formatTime(record.last_refreshed_at)}
                  </Text>
                </div>
              </div>
            </Tooltip>
          );
        }
        return (
          <div>
            <div>{t('刷新成功')}</div>
            <Text type='tertiary'>{formatTime(record.last_refreshed_at)}</Text>
          </div>
        );
      },
    },
    {
      title: t('状态'),
      render: (_, record) => (
        <Tag color={record.enabled ? 'green' : 'red'}>
          {record.enabled ? t('启用') : t('禁用')}
        </Tag>
      ),
    },
    {
      title: t('操作'),
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            loading={refreshingBindingId === record.id}
            disabled={!record.enabled}
            onClick={() => refreshUsage(record)}
          >
            {t('刷新额度')}
          </Button>
          {adminUser && (
            <>
              <Button size='small' onClick={() => openEditModal(record)}>
                {t('编辑')}
              </Button>
              <Button
                size='small'
                type='danger'
                onClick={() => deleteBinding(record)}
              >
                {t('删除')}
              </Button>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Card
        title={t('DeepSeek额度')}
        headerExtraContent={
          <Space>
            <Button loading={refreshingAll} onClick={refreshAllUsage}>
              {t('刷新全部额度')}
            </Button>
            {adminUser && (
              <Button type='primary' onClick={openCreateModal}>
                {t('新增配置')}
              </Button>
            )}
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={bindings}
          loading={bindingLoading}
          pagination={false}
          rowKey='id'
        />
      </Card>

      <Modal
        title={
          quotaForm.id
            ? t('编辑 DeepSeek 额度配置')
            : t('新增 DeepSeek 额度配置')
        }
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={saveBinding}
        confirmLoading={saving}
        style={{ width: 720 }}
      >
        <Form key={quotaForm.id || 'new'} layout='vertical'>
          <Form.Input
            field='name'
            label={t('名称')}
            value={quotaForm.name}
            onChange={(value) =>
              setQuotaForm((current) => ({ ...current, name: value }))
            }
            placeholder={t('例如：DeepSeek 主账号')}
          />
          <Form.TextArea
            field='note'
            label={t('备注')}
            value={quotaForm.note}
            onChange={(value) =>
              setQuotaForm((current) => ({ ...current, note: value }))
            }
            placeholder={t('请输入备注')}
            maxCount={255}
          />
          <Form.TextArea
            field='request_curl'
            label={t('DeepSeek 额度 curl')}
            value={quotaForm.request_curl}
            onChange={(value) =>
              setQuotaForm((current) => ({ ...current, request_curl: value }))
            }
            placeholder={
              quotaForm.id
                ? t('留空则不修改，粘贴新的 curl 可替换')
                : t('粘贴 platform.deepseek.com 额度接口 curl')
            }
            autosize={{ minRows: 5, maxRows: 10 }}
          />
          <div className='flex items-center gap-3'>
            <Text>{t('启用')}</Text>
            <Switch
              checked={quotaForm.enabled}
              onChange={(value) =>
                setQuotaForm((current) => ({ ...current, enabled: value }))
              }
            />
          </div>
        </Form>
      </Modal>
    </div>
  );
}
