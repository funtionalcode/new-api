/*
Copyright (C) 2026 QuantumNous

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

export const getCliproxyRefreshStatus = (record = {}) => {
  const error =
    typeof record.last_error === 'string' ? record.last_error.trim() : '';

  return {
    failed: error !== '',
    labelKey: error ? '刷新失败' : '刷新成功',
    error,
    refreshedAt: record.last_refreshed_at || 0,
  };
};

const normalizeProviderValue = (value) => {
  if (typeof value !== 'string') return '';
  return value.trim().toLowerCase().replace(/[_\s]/g, '-');
};

const pickProvider = (value) => {
  const normalized = normalizeProviderValue(value);
  if (!normalized) return '';
  if (normalized === 'codex' || normalized.startsWith('codex-')) return 'codex';
  if (normalized === 'claude' || normalized.startsWith('claude-'))
    return 'claude';
  return '';
};

export const getCliproxyAuthProvider = (record = {}) => {
  const explicitProvider =
    pickProvider(record.provider) ||
    pickProvider(record.type) ||
    pickProvider(record.auth_type) ||
    pickProvider(record.authType);
  if (explicitProvider) return explicitProvider;

  const fields = [
    record.auth_index,
    record.authIndex,
    record.auth_name,
    record.authName,
    record.name,
    record.auth_file,
    record.authFile,
  ];
  for (const field of fields) {
    const provider = pickProvider(field);
    if (provider) return provider;
  }
  return 'unknown';
};
