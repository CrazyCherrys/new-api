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
import { Card, Space, SideSheet } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import ResolutionPicker from './ResolutionPicker';
import AspectRatioGrid from './AspectRatioGrid';
import ImageUploader from './ImageUploader';
import BatchSubmitter from './BatchSubmitter';

const ParameterSidebar = ({
  resolution,
  aspectRatio,
  referenceImage,
  count,
  onResolutionChange,
  onAspectRatioChange,
  onReferenceImageChange,
  onCountChange,
  onSubmit,
  loading,
  disabled,
  isMobile,
  visible,
  onClose,
}) => {
  const { t } = useTranslation();

  const content = (
    <Space vertical spacing="large" className="w-full">
      <ResolutionPicker
        value={resolution}
        onChange={onResolutionChange}
        disabled={disabled || loading}
      />

      <AspectRatioGrid
        value={aspectRatio}
        onChange={onAspectRatioChange}
        disabled={disabled || loading}
      />

      <ImageUploader
        value={referenceImage}
        onChange={onReferenceImageChange}
        disabled={disabled || loading}
      />

      <BatchSubmitter
        count={count}
        onCountChange={onCountChange}
        onSubmit={onSubmit}
        loading={loading}
        disabled={disabled}
      />
    </Space>
  );

  if (isMobile) {
    return (
      <SideSheet
        title={t('生成参数')}
        visible={visible}
        onCancel={onClose}
        placement="bottom"
        height={600}
      >
        <div className="p-4">{content}</div>
      </SideSheet>
    );
  }

  return (
    <Card className="w-full h-full overflow-y-auto">
      {content}
    </Card>
  );
};

export default ParameterSidebar;
