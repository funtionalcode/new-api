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

import React, { useState } from 'react';
import { Modal, Form } from '@douyinfe/semi-ui';
import { API, showError } from '../../../helpers';

const SearchModal = ({
  searchModalVisible,
  handleSearchConfirm,
  handleCloseModal,
  isMobile,
  isAdminUser,
  inputs,
  dataExportDefaultTime,
  timeOptions,
  handleInputChange,
  t,
}) => {
  const [userOptions, setUserOptions] = useState([]);

  const FORM_FIELD_PROPS = {
    className: 'w-full mb-2 !rounded-lg',
  };

  const createFormField = (Component, props) => (
    <Component {...FORM_FIELD_PROPS} {...props} />
  );

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

  const { start_timestamp, end_timestamp, username } = inputs;

  return (
    <Modal
      title={t('搜索条件')}
      visible={searchModalVisible}
      onOk={handleSearchConfirm}
      onCancel={handleCloseModal}
      closeOnEsc={true}
      size={isMobile ? 'full-width' : 'small'}
      centered
    >
      <Form layout='vertical' className='w-full'>
        {createFormField(Form.DatePicker, {
          field: 'start_timestamp',
          label: t('起始时间'),
          initValue: start_timestamp,
          value: start_timestamp,
          type: 'dateTime',
          name: 'start_timestamp',
          onChange: (value) => handleInputChange(value, 'start_timestamp'),
        })}

        {createFormField(Form.DatePicker, {
          field: 'end_timestamp',
          label: t('结束时间'),
          initValue: end_timestamp,
          value: end_timestamp,
          type: 'dateTime',
          name: 'end_timestamp',
          onChange: (value) => handleInputChange(value, 'end_timestamp'),
        })}

        {createFormField(Form.Select, {
          field: 'data_export_default_time',
          label: t('时间粒度'),
          initValue: dataExportDefaultTime,
          placeholder: t('时间粒度'),
          name: 'data_export_default_time',
          optionList: timeOptions,
          onChange: (value) =>
            handleInputChange(value, 'data_export_default_time'),
        })}

        {isAdminUser &&
          createFormField(Form.Select, {
            field: 'username',
            label: t('用户名称'),
            value: username,
            placeholder: t('搜索并选择用户'),
            name: 'username',
            filter: true,
            remote: true,
            optionList: userOptions,
            onSearch: searchUsers,
            onChange: (value) => handleInputChange(value, 'username'),
          })}
      </Form>
    </Modal>
  );
};

export default SearchModal;
