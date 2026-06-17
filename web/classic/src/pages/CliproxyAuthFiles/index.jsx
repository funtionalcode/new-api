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
import {
  Button,
  Card,
  Form,
  Modal,
  Progress,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, isAdmin, isRoot, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const getAuthIndex = (authFile) =>
  authFile?.authIndex || authFile?.auth_index || '';
const getAuthName = (authFile) =>
  authFile?.name || authFile?.authName || authFile?.auth_name || '';
const getAuthFileContent = (authFile) =>
  authFile?.authFile || authFile?.auth_file || '';
const getAccountId = (authFile) =>
  authFile?.accountId || authFile?.account_id || '';
const getPlanType = (authFile) =>
  authFile?.planType || authFile?.plan_type || authFile?.last_plan_type || '';
const getAuthRemark = (authFile) =>
  authFile?.description || authFile?.remark || '';

const normalizePlanKey = (value) => {
  if (typeof value !== 'string') return '';
  return value
    .trim()
    .toLowerCase()
    .replace(/[-_\s]/g, '');
};

const getPlanTagConfig = (value) => {
  const key = normalizePlanKey(value);
  if (!key) return null;
  if (key === 'pro' || key === 'pro20x')
    return { label: 'Pro 20x', color: 'orange' };
  if (key === 'planmax' || key === 'claudemax')
    return { label: 'Claude Max', color: 'orange' };
  if (key === 'planpro' || key === 'claudepro')
    return { label: 'Claude Plus', color: 'purple' };
  if (key === 'prolite' || key === 'pro5x')
    return { label: 'Pro 5x', color: 'cyan' };
  if (key === 'team') return { label: 'Team', color: 'green' };
  if (key === 'planteam' || key === 'claudeteam')
    return { label: 'Claude Team', color: 'green' };
  if (key === 'plus') return { label: 'Plus', color: 'purple' };
  if (key === 'free') return { label: 'Free', color: 'grey' };
  if (key === 'planfree' || key === 'claudefree')
    return { label: 'Claude Free', color: 'grey' };
  if (key === 'claude') return { label: 'Claude', color: 'orange' };
  return { label: value, color: 'white' };
};

const renderPlanTag = (value) => {
  const config = getPlanTagConfig(value);
  if (!config) return '-';
  return (
    <Tag color={config.color} shape='circle'>
      {config.label}
    </Tag>
  );
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

const renderUsageLimit = (percent, resetAt) => {
  const normalizedPercent = normalizeUsagePercent(percent);

  return (
    <Tooltip content={`${normalizedPercent}%/100%`}>
      <div style={{ minWidth: 120 }}>
        <Progress
          percent={normalizedPercent}
          stroke={getUsageColor(normalizedPercent)}
          format={() => `${normalizedPercent}%`}
          style={{ marginBottom: 2 }}
        />
        <Text type='tertiary'>{formatTime(resetAt)}</Text>
      </div>
    </Tooltip>
  );
};

const emptyBindingForm = {
  id: undefined,
  user_id: undefined,
  username: '',
  auth_index: '',
  auth_name: '',
  auth_file: '',
  description: '',
  account_id: '',
  last_plan_type: '',
  enabled: true,
};

const buildBindingForm = (binding = emptyBindingForm) => ({
  id: binding.id,
  user_id: binding.user_id,
  username: binding.username || '',
  auth_index: binding.auth_index || '',
  auth_name: binding.auth_name || '',
  auth_file: binding.auth_file || '',
  description: binding.description || '',
  account_id: binding.account_id || '',
  last_plan_type: binding.last_plan_type || '',
  enabled: binding.enabled !== false,
});

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const normalizeRemoteAuthFiles = (response) => {
  const data = response?.data;
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.items)) return data.items;
  if (Array.isArray(data?.authFiles)) return data.authFiles;
  if (Array.isArray(data?.auth_files)) return data.auth_files;
  return [];
};

export default function CliproxyAuthFiles() {
  const { t } = useTranslation();
  const [options, setOptions] = useState({
    CliproxyAPIBaseURL: '',
    CliproxyAPIPassword: '',
  });
  const [remoteFiles, setRemoteFiles] = useState([]);
  const [bindings, setBindings] = useState([]);
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [remoteLoading, setRemoteLoading] = useState(false);
  const [bindingLoading, setBindingLoading] = useState(false);
  const [refreshingBindingId, setRefreshingBindingId] = useState(undefined);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [savingConfig, setSavingConfig] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [bindingForm, setBindingForm] = useState(emptyBindingForm);
  const rootUser = isRoot();
  const adminUser = isAdmin();

  const remoteFileOptions = useMemo(
    () =>
      remoteFiles.map((authFile) => ({
        label: `${getAuthName(authFile) || '-'} (${getAuthIndex(authFile) || '-'})`,
        value: getAuthIndex(authFile),
        authFile,
      })),
    [remoteFiles],
  );

  const userOptions = useMemo(
    () =>
      users.map((user) => ({
        label: `${user.username || '-'} (ID: ${user.id})`,
        value: user.id,
        username: user.username || '',
      })),
    [users],
  );

  const loadOptions = async () => {
    try {
      const res = await API.get('/api/option/');
      if (res.data.success) {
        setOptions({
          CliproxyAPIBaseURL: res.data.data?.CliproxyAPIBaseURL || '',
          CliproxyAPIPassword: '',
        });
      } else {
        showError(res.data.message || t('加载配置失败'));
      }
    } catch (error) {
      showError(error);
    }
  };

  const loadRemoteFiles = async () => {
    setRemoteLoading(true);
    try {
      const res = await API.get('/api/cliproxy/auth-files/remote');
      if (res.data.success) {
        setRemoteFiles(normalizeRemoteAuthFiles(res.data));
      } else {
        showError(res.data.message || t('拉取远端认证文件失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setRemoteLoading(false);
    }
  };

  const loadBindings = async () => {
    setBindingLoading(true);
    try {
      const res = await API.get('/api/cliproxy/auth-files/bindings', {
        params: { p: 1, page_size: 100 },
      });
      if (res.data.success) {
        setBindings(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('加载绑定失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setBindingLoading(false);
    }
  };

  const searchUsers = async (keyword) => {
    if (!keyword) return;
    try {
      const res = await API.get('/api/user/search', {
        params: { keyword, group: '', p: 1, page_size: 20 },
      });
      if (res.data.success) {
        setUsers(res.data.data?.items || []);
      } else {
        showError(res.data.message || t('搜索用户失败'));
      }
    } catch (error) {
      showError(error);
    }
  };

  const saveConfig = async () => {
    setSavingConfig(true);
    try {
      const updateRequests = [
        API.put('/api/option/', {
          key: 'CliproxyAPIBaseURL',
          value: options.CliproxyAPIBaseURL || '',
        }),
      ];
      if (options.CliproxyAPIPassword.trim()) {
        updateRequests.push(
          API.put('/api/option/', {
            key: 'CliproxyAPIPassword',
            value: options.CliproxyAPIPassword,
          }),
        );
      }
      const responses = await Promise.all(updateRequests);
      const failed = responses.find((res) => !res.data.success);
      if (failed) {
        showError(failed.data.message || t('保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      setOptions((current) => ({ ...current, CliproxyAPIPassword: '' }));
    } catch (error) {
      showError(error);
    } finally {
      setSavingConfig(false);
    }
  };

  const openCreateModal = (remoteFile) => {
    setBindingForm(
      buildBindingForm({
        auth_index: getAuthIndex(remoteFile),
        auth_name: getAuthName(remoteFile),
        auth_file: getAuthFileContent(remoteFile),
        description: getAuthRemark(remoteFile),
        account_id: getAccountId(remoteFile),
        last_plan_type: getPlanType(remoteFile),
        enabled: remoteFile?.enabled !== false,
      }),
    );
    setModalVisible(true);
  };

  const openEditModal = (binding) => {
    setBindingForm(buildBindingForm(binding));
    setUsers(
      binding.user_id
        ? [{ id: binding.user_id, username: binding.username || '' }]
        : [],
    );
    setModalVisible(true);
  };

  const saveBinding = async () => {
    if (!bindingForm.user_id) {
      showError(t('请选择用户'));
      return;
    }
    if (!bindingForm.auth_index.trim()) {
      showError(t('请选择认证文件'));
      return;
    }

    setLoading(true);
    try {
      const payload = {
        user_id: bindingForm.user_id,
        auth_index: bindingForm.auth_index.trim(),
        auth_name: bindingForm.auth_name.trim(),
        auth_file: bindingForm.auth_file.trim(),
        description: bindingForm.description.trim(),
        account_id: bindingForm.account_id.trim(),
        last_plan_type: bindingForm.last_plan_type.trim(),
        enabled: bindingForm.enabled,
      };
      const res = bindingForm.id
        ? await API.put(
            `/api/cliproxy/auth-files/bindings/${bindingForm.id}`,
            payload,
          )
        : await API.post('/api/cliproxy/auth-files/bindings', payload);
      if (res.data.success) {
        showSuccess(t('保存成功'));
        setModalVisible(false);
        setBindingForm(emptyBindingForm);
        await loadBindings();
      } else {
        showError(res.data.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const deleteBinding = (binding) => {
    Modal.confirm({
      title: t('确认删除绑定？'),
      content: `${binding.username || '-'} / ${binding.auth_index}`,
      onOk: async () => {
        try {
          const res = await API.delete(
            `/api/cliproxy/auth-files/bindings/${binding.id}`,
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
        `/api/cliproxy/auth-files/bindings/${binding.id}/refresh-usage`,
      );
      if (res.data.success) {
        showSuccess(t('刷新成功'));
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

  const renderRemarkInput = () => (
    <Form.TextArea
      field='description'
      label={t('备注')}
      value={bindingForm.description}
      onChange={(value) =>
        setBindingForm((current) => ({
          ...current,
          description: value,
        }))
      }
      placeholder={t('请输入备注')}
      maxCount={255}
    />
  );

  const refreshAllUsage = async () => {
    if (bindings.length === 0) {
      showError(t('没有需要刷新的绑定'));
      return;
    }
    setRefreshingAll(true);
    let successCount = 0;
    let failCount = 0;
    try {
      const results = await Promise.allSettled(
        bindings.map((binding) =>
          API.post(
            `/api/cliproxy/auth-files/bindings/${binding.id}/refresh-usage`,
          ),
        ),
      );
      results.forEach((result) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
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
    if (rootUser) {
      loadOptions();
    }
  }, [rootUser]);

  const remoteColumns = [
    { title: t('认证文件'), dataIndex: 'name' },
    {
      title: t('索引'),
      render: (_, record) => record.authIndex || record.auth_index || '-',
    },
    {
      title: t('账号'),
      render: (_, record) => record.accountId || record.account_id || '-',
    },
    {
      title: t('备注'),
      render: (_, record) => {
        const remark = getAuthRemark(record);
        return remark ? (
          <Tooltip content={remark}>
            <div className='max-w-[160px] truncate'>{remark}</div>
          </Tooltip>
        ) : (
          '-'
        );
      },
    },
    {
      title: t('套餐'),
      render: (_, record) => renderPlanTag(getPlanType(record)),
    },
    {
      title: t('绑定状态'),
      render: (_, record) => {
        const authIndex = record.authIndex || record.auth_index || '';
        const binding = bindings.find((b) => b.auth_index === authIndex);
        if (binding) {
          return (
            <Tag color='green' shape='circle'>
              {t('已绑定')} {binding.username ? `(${binding.username})` : ''}
            </Tag>
          );
        }
        return (
          <Tag color='grey' shape='circle'>
            {t('未绑定')}
          </Tag>
        );
      },
    },
    {
      title: t('状态'),
      render: (_, record) => (
        <Tag color={record.enabled === false ? 'red' : 'green'}>
          {record.enabled === false ? t('禁用') : t('启用')}
        </Tag>
      ),
    },
    ...(adminUser
      ? [
          {
            title: t('操作'),
            render: (_, record) => (
              <Button size='small' onClick={() => openCreateModal(record)}>
                {t('绑定用户')}
              </Button>
            ),
          },
        ]
      : []),
  ];

  const bindingColumns = [
    {
      title: t('用户'),
      render: (_, record) => (
        <div>
          <div>{record.username || '-'}</div>
          <Text type='tertiary'>ID: {record.user_id}</Text>
        </div>
      ),
    },
    { title: t('认证文件'), dataIndex: 'auth_name' },
    {
      title: t('备注'),
      render: (_, record) =>
        record.description ? (
          <Tooltip content={record.description}>
            <div className='max-w-[160px] truncate'>{record.description}</div>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: t('套餐'),
      render: (_, record) => renderPlanTag(record.last_plan_type),
    },
    {
      title: t('5小时限额'),
      render: (_, record) =>
        renderUsageLimit(
          record.last_five_hour_percent,
          record.last_five_hour_reset_at,
        ),
    },
    {
      title: t('周限额'),
      render: (_, record) =>
        renderUsageLimit(
          record.last_weekly_percent,
          record.last_weekly_reset_at,
        ),
    },
    {
      title: t('Codex 5小时限额'),
      render: (_, record) =>
        renderUsageLimit(
          record.last_codex_five_hour_percent,
          record.last_codex_five_hour_reset_at,
        ),
    },
    {
      title: t('Codex 周限额'),
      render: (_, record) =>
        renderUsageLimit(
          record.last_codex_weekly_percent,
          record.last_codex_weekly_reset_at,
        ),
    },
    {
      title: t('刷新进度'),
      render: (_, record) => (
        <div>
          <div>{record.last_error ? t('刷新失败') : t('刷新成功')}</div>
          <Text type='tertiary'>{formatTime(record.last_refreshed_at)}</Text>
        </div>
      ),
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
        <div className='flex gap-2'>
          <Button
            size='small'
            loading={refreshingBindingId === record.id}
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
        </div>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <div className='space-y-4'>
        {rootUser && (
          <Card title={t('Cliproxy API 配置')}>
            <Form layout='horizontal'>
              <Form.Input
                field='baseURL'
                label={t('Cliproxy API 地址')}
                value={options.CliproxyAPIBaseURL}
                onChange={(value) =>
                  setOptions((current) => ({
                    ...current,
                    CliproxyAPIBaseURL: value,
                  }))
                }
                placeholder='http://127.0.0.1:8317'
              />
              <Form.Input
                field='password'
                mode='password'
                label={t('Cliproxy API 登录密码')}
                value={options.CliproxyAPIPassword}
                onChange={(value) =>
                  setOptions((current) => ({
                    ...current,
                    CliproxyAPIPassword: value,
                  }))
                }
                placeholder={t('留空则不修改')}
              />
              <Button
                type='primary'
                loading={savingConfig}
                onClick={saveConfig}
              >
                {t('保存配置')}
              </Button>
            </Form>
          </Card>
        )}

        {adminUser && (
          <Card
            title={t('远端认证文件')}
            headerExtraContent={
              <Button loading={remoteLoading} onClick={loadRemoteFiles}>
                {t('拉取远端列表')}
              </Button>
            }
          >
            <Table
              columns={remoteColumns}
              dataSource={remoteFiles}
              loading={remoteLoading}
              pagination={false}
              rowKey={(record) => record.authIndex || record.auth_index}
            />
          </Card>
        )}

        <Card
          title={t('认证文件绑定')}
          headerExtraContent={
            <div className='flex gap-2'>
              {adminUser && (
                <Button loading={refreshingAll} onClick={refreshAllUsage}>
                  {t('刷新全部额度')}
                </Button>
              )}
              {adminUser && (
                <Button type='primary' onClick={() => openCreateModal()}>
                  {t('新增绑定')}
                </Button>
              )}
            </div>
          }
        >
          <Table
            columns={bindingColumns}
            dataSource={bindings}
            loading={bindingLoading}
            pagination={false}
            rowKey='id'
          />
        </Card>
      </div>

      <Modal
        title={bindingForm.id ? t('编辑备注') : t('新增绑定')}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={saveBinding}
        confirmLoading={loading}
      >
        <Form layout='vertical'>
          {bindingForm.id ? (
            <>
              <Form.Input
                field='username_display'
                label={t('用户')}
                value={bindingForm.username || '-'}
                disabled
              />
              {renderRemarkInput()}
            </>
          ) : bindingForm.auth_index ? (
            <>
              <Form.Select
                field='user_id'
                label={t('用户')}
                filter
                remote
                value={bindingForm.user_id}
                optionList={userOptions}
                onSearch={searchUsers}
                onChange={(value) => {
                  const user = userOptions.find(
                    (option) => option.value === value,
                  );
                  setBindingForm((current) => ({
                    ...current,
                    user_id: value,
                    username: user?.username || current.username,
                  }));
                }}
                placeholder={t('搜索并选择用户')}
              />
              {renderRemarkInput()}
            </>
          ) : (
            <>
              <Form.Select
                field='auth_index'
                label={t('认证文件')}
                filter
                value={bindingForm.auth_index}
                optionList={remoteFileOptions}
                onChange={(value) => {
                  const selected = remoteFileOptions.find(
                    (option) => option.value === value,
                  );
                  const authFile = selected?.authFile;
                  setBindingForm((current) => ({
                    ...current,
                    auth_index: value,
                    auth_name: getAuthName(authFile),
                    auth_file: getAuthFileContent(authFile),
                    description: getAuthRemark(authFile),
                    account_id: getAccountId(authFile),
                    enabled: authFile?.enabled !== false,
                  }));
                }}
                placeholder={t('请选择认证文件')}
              />
              <Form.Select
                field='user_id'
                label={t('用户')}
                filter
                remote
                value={bindingForm.user_id}
                optionList={userOptions}
                onSearch={searchUsers}
                onChange={(value) => {
                  const user = userOptions.find(
                    (option) => option.value === value,
                  );
                  setBindingForm((current) => ({
                    ...current,
                    user_id: value,
                    username: user?.username || current.username,
                  }));
                }}
                placeholder={t('搜索并选择用户')}
              />
              {renderRemarkInput()}
            </>
          )}
        </Form>
      </Modal>
    </div>
  );
}
