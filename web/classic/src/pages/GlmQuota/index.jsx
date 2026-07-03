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

const planSpecs = [
  {
    value: '标准版',
    label: '标准版',
    fiveHourLimitTokens: 60000000,
    weeklyLimitTokens: 300000000,
  },
  {
    value: '高级版',
    label: '高级版',
    fiveHourLimitTokens: 160000000,
    weeklyLimitTokens: 800000000,
  },
];

const emptyQuotaForm = {
  id: undefined,
  name: '',
  note: '',
  request_curl: '',
  proxy: '',
  plan_type: '',
  five_hour_limit_tokens: 0,
  weekly_limit_tokens: 0,
  enabled: true,
};

const buildQuotaForm = (binding = emptyQuotaForm) => ({
  id: binding.id,
  name: binding.name || '',
  note: binding.note || '',
  request_curl: binding.request_curl || '',
  proxy: binding.proxy || '',
  plan_type: binding.plan_type || '',
  five_hour_limit_tokens: Number(binding.five_hour_limit_tokens || 0),
  weekly_limit_tokens: Number(binding.weekly_limit_tokens || 0),
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

const renderPlanTag = (value) => {
  const plan = String(value || '').trim();
  if (!plan) return '-';
  return (
    <Tag color='violet' shape='circle'>
      {plan}
    </Tag>
  );
};

const getPlanSpec = (value) =>
  planSpecs.find((spec) => spec.value === value || spec.label === value);

const parseModelSummary = (value) => {
  if (Array.isArray(value)) return value;
  if (typeof value !== 'string' || !value.trim()) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
};

const renderModelSummary = (value, callCount) => {
  const summary = parseModelSummary(value);
  if (summary.length === 0) {
    return callCount ? `${formatTokens(callCount)} 次调用` : '-';
  }
  const content = summary
    .map(
      (item) =>
        `${item.modelName || item.model_name || '-'}: ${formatTokens(
          item.totalTokens || item.total_tokens,
        )}`,
    )
    .join('\n');
  return (
    <Tooltip content={<pre style={{ margin: 0 }}>{content}</pre>}>
      <div className='max-w-[180px] truncate'>
        {summary
          .slice(0, 2)
          .map((item) => item.modelName || item.model_name || '-')
          .join('、')}
      </div>
    </Tooltip>
  );
};

const renderTokenLimit = (used, limit, percent) => {
  const normalizedPercent = normalizeUsagePercent(percent);
  const hasLimit = Number(limit || 0) > 0;
  const remaining = hasLimit
    ? Math.max(0, Number(limit || 0) - Number(used || 0))
    : 0;
  const display = hasLimit
    ? `${formatTokens(used)} / ${formatTokens(limit)}`
    : `${formatTokens(used)} / 未配置`;

  return (
    <Tooltip content={display}>
      <div style={{ minWidth: 150 }}>
        <Progress
          percent={hasLimit ? normalizedPercent : 0}
          stroke={getUsageColor(normalizedPercent)}
          format={() => (hasLimit ? `${normalizedPercent}%` : '-')}
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

export default function GlmQuota() {
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
      const res = await API.get('/api/glm-quota/bindings', {
        params: { p: 1, page_size: 100 },
      });
      if (res.data.success) {
        setBindings(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('加载 GLM 额度配置失败'));
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
      showError(t('请输入 BigModel 用量 curl'));
      return;
    }
    if (!quotaForm.plan_type.trim()) {
      showError(t('请选择套餐规格'));
      return;
    }
    if (
      Number(quotaForm.five_hour_limit_tokens || 0) <= 0 ||
      Number(quotaForm.weekly_limit_tokens || 0) <= 0
    ) {
      showError(t('套餐规格额度不能为空'));
      return;
    }
    setSaving(true);
    try {
      const payload = {
        name: quotaForm.name.trim(),
        note: quotaForm.note.trim(),
        request_curl: quotaForm.request_curl.trim(),
        proxy: quotaForm.proxy.trim(),
        plan_type: quotaForm.plan_type.trim(),
        five_hour_limit_tokens: Number(
          quotaForm.five_hour_limit_tokens || 0,
        ),
        weekly_limit_tokens: Number(quotaForm.weekly_limit_tokens || 0),
        enabled: quotaForm.enabled,
      };
      const res = quotaForm.id
        ? await API.put(`/api/glm-quota/bindings/${quotaForm.id}`, payload)
        : await API.post('/api/glm-quota/bindings', payload);
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
      title: t('确认删除 GLM 额度配置？'),
      content: binding.name || '-',
      onOk: async () => {
        try {
          const res = await API.delete(`/api/glm-quota/bindings/${binding.id}`);
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
        `/api/glm-quota/bindings/${binding.id}/refresh-usage`,
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
      showError(t('没有需要刷新的 GLM 额度配置'));
      return;
    }
    setRefreshingAll(true);
    let successCount = 0;
    let failCount = 0;
    try {
      const results = await Promise.allSettled(
        enabledBindings.map((binding) =>
          API.post(`/api/glm-quota/bindings/${binding.id}/refresh-usage`),
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
      title: t('套餐规格'),
      render: (_, record) => renderPlanTag(record.plan_type),
    },
    {
      title: t('5小时额度'),
      render: (_, record) =>
        renderTokenLimit(
          record.last_five_hour_used_tokens,
          record.five_hour_limit_tokens,
          record.last_five_hour_percent,
        ),
    },
    {
      title: t('周额度'),
      render: (_, record) =>
        renderTokenLimit(
          record.last_weekly_used_tokens,
          record.weekly_limit_tokens,
          record.last_weekly_percent,
        ),
    },
    {
      title: t('模型汇总'),
      render: (_, record) =>
        renderModelSummary(
          record.last_model_summary,
          record.last_model_call_count,
        ),
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
        title={t('GLM额度')}
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
        title={quotaForm.id ? t('编辑 GLM 额度配置') : t('新增 GLM 额度配置')}
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
            placeholder={t('例如：主账号 / 团队额度')}
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
          <Form.Select
            field='plan_type'
            label={t('套餐规格')}
            value={quotaForm.plan_type}
            optionList={planSpecs.map((spec) => ({
              label: `${spec.label}（5小时 ${formatTokens(
                spec.fiveHourLimitTokens,
              )} / 每周 ${formatTokens(spec.weeklyLimitTokens)}）`,
              value: spec.value,
            }))}
            onChange={(value) =>
              setQuotaForm((current) => {
                const spec = getPlanSpec(value);
                return {
                  ...current,
                  plan_type: value,
                  five_hour_limit_tokens:
                    spec?.fiveHourLimitTokens ||
                    current.five_hour_limit_tokens,
                  weekly_limit_tokens:
                    spec?.weeklyLimitTokens || current.weekly_limit_tokens,
                };
              })
            }
            placeholder={t('请选择套餐规格')}
          />
          <Form.InputNumber
            field='five_hour_limit_tokens'
            label={t('5小时额度 Token')}
            min={0}
            value={quotaForm.five_hour_limit_tokens}
            onChange={(value) =>
              setQuotaForm((current) => ({
                ...current,
                five_hour_limit_tokens: Number(value || 0),
              }))
            }
          />
          <Form.InputNumber
            field='weekly_limit_tokens'
            label={t('周额度 Token')}
            min={0}
            value={quotaForm.weekly_limit_tokens}
            onChange={(value) =>
              setQuotaForm((current) => ({
                ...current,
                weekly_limit_tokens: Number(value || 0),
              }))
            }
          />
          <Form.TextArea
            field='request_curl'
            label={t('BigModel 用量 curl')}
            value={quotaForm.request_curl}
            onChange={(value) =>
              setQuotaForm((current) => ({ ...current, request_curl: value }))
            }
            placeholder={
              quotaForm.id
                ? t('留空则不修改，粘贴新的 curl 可替换')
                : t('粘贴 bigmodel.cn 用量接口 curl')
            }
            autosize={{ minRows: 5, maxRows: 10 }}
          />
          <Form.Input
            field='proxy'
            label={t('代理地址')}
            value={quotaForm.proxy}
            onChange={(value) =>
              setQuotaForm((current) => ({ ...current, proxy: value }))
            }
            placeholder={t('例如: socks5://user:pass@host:port')}
            showClear
            extraText={t('用于配置网络代理，支持 socks5 协议')}
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
