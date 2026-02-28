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
import { Button, Space, ImagePreview, Modal } from '@douyinfe/semi-ui';
import {
  IconDownload,
  IconEyeOpened,
  IconDelete,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { showSuccess, showError } from '../../../helpers';

const ImageActionGroup = ({ task, onRegenerate, onDelete }) => {
  const { t } = useTranslation();
  const [previewVisible, setPreviewVisible] = useState(false);
  const [deleteModalVisible, setDeleteModalVisible] = useState(false);

  // 下载图片
  const handleDownload = async () => {
    if (!task.result_url) {
      showError(t('图片地址不存在'));
      return;
    }

    try {
      const response = await fetch(task.result_url);
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `image_${task.id}_${Date.now()}.png`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      showSuccess(t('图片下载成功'));
    } catch (error) {
      console.error('Download error:', error);
      showError(t('图片下载失败'));
    }
  };

  // 预览图片
  const handlePreview = () => {
    if (!task.result_url) {
      showError(t('图片地址不存在'));
      return;
    }
    setPreviewVisible(true);
  };

  // 删除任务
  const handleDelete = () => {
    setDeleteModalVisible(true);
  };

  const confirmDelete = () => {
    if (onDelete) {
      onDelete(task.id);
    }
    setDeleteModalVisible(false);
  };

  // 重新生成
  const handleRegenerate = () => {
    if (onRegenerate) {
      onRegenerate(task);
    }
  };

  const hasResult = task.status === 'succeeded' && task.result_url;

  return (
    <>
      <Space spacing="tight">
        {hasResult && (
          <>
            <Button
              icon={<IconEyeOpened />}
              type="tertiary"
              size="small"
              onClick={handlePreview}
              aria-label={t('预览')}
            />
            <Button
              icon={<IconDownload />}
              type="tertiary"
              size="small"
              onClick={handleDownload}
              aria-label={t('下载')}
            />
          </>
        )}
        <Button
          icon={<IconRefresh />}
          type="tertiary"
          size="small"
          onClick={handleRegenerate}
          aria-label={t('重新生成')}
        />
        <Button
          icon={<IconDelete />}
          type="tertiary"
          size="small"
          onClick={handleDelete}
          aria-label={t('删除')}
        />
      </Space>

      {/* 图片预览 */}
      {hasResult && (
        <ImagePreview
          src={task.result_url}
          visible={previewVisible}
          onVisibleChange={setPreviewVisible}
        />
      )}

      {/* 删除确认对话框 */}
      <Modal
        title={t('确认删除')}
        visible={deleteModalVisible}
        onOk={confirmDelete}
        onCancel={() => setDeleteModalVisible(false)}
        okText={t('删除')}
        cancelText={t('取消')}
        okButtonProps={{ type: 'danger' }}
      >
        {t('确定要删除这个任务吗？此操作无法撤销。')}
      </Modal>
    </>
  );
};

export default ImageActionGroup;
