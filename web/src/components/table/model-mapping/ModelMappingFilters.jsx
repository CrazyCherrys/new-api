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
import { Form, Select, Button, Space } from '@douyinfe/semi-ui';
import { IconSearch, IconPlus, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const ModelMappingFilters = ({
  searchKeyword,
  searchModelType,
  setSearchKeyword,
  setSearchModelType,
  handleSearch,
  openEditModal,
  refresh,
  loading,
}) => {
  const { t } = useTranslation();

  const modelTypeOptions = [
    { value: 0, label: t('全部类型') },
    { value: 1, label: t('对话') },
    { value: 2, label: t('绘画') },
    { value: 3, label: t('视频') },
    { value: 4, label: t('音频') },
  ];

  return (
    <Space>
      <Form.Input
        field='keyword'
        placeholder={t('搜索模型ID或名称')}
        value={searchKeyword}
        onChange={(value) => setSearchKeyword(value)}
        onEnterPress={handleSearch}
        style={{ width: 200 }}
      />
      <Select
        field='model_type'
        placeholder={t('模型类型')}
        value={searchModelType}
        onChange={(value) => setSearchModelType(value)}
        optionList={modelTypeOptions}
        style={{ width: 150 }}
      />
      <Button
        theme='solid'
        type='primary'
        icon={<IconSearch />}
        onClick={handleSearch}
      >
        {t('搜索')}
      </Button>
      <Button
        theme='light'
        type='primary'
        icon={<IconPlus />}
        onClick={() => openEditModal(null)}
      >
        {t('添加')}
      </Button>
      <Button
        theme='light'
        type='secondary'
        icon={<IconRefresh />}
        onClick={refresh}
        loading={loading}
      >
        {t('刷新')}
      </Button>
    </Space>
  );
};

export default ModelMappingFilters;
