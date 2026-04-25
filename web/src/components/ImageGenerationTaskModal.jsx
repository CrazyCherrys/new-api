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
import { Modal, Button, Typography, Space, Spin, Divider } from '@douyinfe/semi-ui';
import { IconDownload, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../helpers';

const { Text, Title } = Typography;

const ImageGenerationTaskModal = ({ visible, onClose, task, onRetrySuccess }) => {
  const { t } = useTranslation();
  const [retrying, setRetrying] = useState(false);

  if (!task) return null;

  const isSuccess = task.status === 'success';
  const isFailed = task.status === 'failed';

  // 解析元数据
  let metadata = {};
  try {
    if (task.image_metadata) {
      metadata = JSON.parse(task.image_metadata);
    }
  } catch (e) {
    console.error('Failed to parse image metadata:', e);
  }

  // 解析参数
  let params = {};
  try {
    if (task.params) {
      params = JSON.parse(task.params);
    }
  } catch (e) {
    console.error('Failed to parse params:', e);
  }

  // 格式化时间
  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    return new Date(timestamp * 1000).toLocaleString();
  };

  // 下载图片
  const handleDownload = () => {
    if (!task.image_url) return;

    const link = document.createElement('a');
    link.href = task.image_url;
    link.download = `image-${task.id}.png`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  // 重试生成
  const handleRetry = async () => {
    setRetrying(true);
    try {
      const response = await API.post(`/api/image-generation/tasks/${task.id}/retry`);
      if (response.data.success) {
        showSuccess(t('重试请求已提交'));
        if (onRetrySuccess) {
          onRetrySuccess(response.data.data);
        }
        onClose();
      } else {
        showError(response.data.message || t('重试失败'));
      }
    } catch (error) {
      showError(error.message || t('重试失败'));
    } finally {
      setRetrying(false);
    }
  };

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      width={800}
      bodyStyle={{ padding: '24px' }}
      title={t('任务详情')}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        {/* 成功状态 - 显示图片 */}
        {isSuccess && task.image_url && (
          <div style={{ textAlign: 'center', marginBottom: '16px' }}>
            <img
              src={task.image_url}
              alt="Generated"
              style={{
                maxWidth: '100%',
                maxHeight: '400px',
                borderRadius: '8px',
                boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
              }}
            />
          </div>
        )}

        {/* 失败状态 - 显示错误信息 */}
        {isFailed && task.error_message && (
          <div
            style={{
              padding: '12px',
              backgroundColor: '#fef0f0',
              border: '1px solid #fde2e2',
              borderRadius: '4px',
              marginBottom: '16px',
            }}
          >
            <Text strong style={{ color: '#f56c6c' }}>
              {t('错误信息')}:
            </Text>
            <div style={{ marginTop: '8px' }}>
              <Text style={{ color: '#f56c6c', whiteSpace: 'pre-wrap' }}>
                {task.error_message}
              </Text>
            </div>
          </div>
        )}

        {/* 基本信息 */}
        <div>
          <Text strong style={{ fontSize: '14px' }}>
            {t('提示词')}:
          </Text>
          <div
            style={{
              marginTop: '8px',
              padding: '12px',
              backgroundColor: '#f8f9fa',
              borderRadius: '4px',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            <Text>{task.prompt}</Text>
          </div>
        </div>

        <Divider margin="8px" />

        {/* 详细信息 */}
        <Space vertical spacing="loose" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <Text type="tertiary">{t('模型ID')}:</Text>
            <Text>{task.model_id}</Text>
          </div>

          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <Text type="tertiary">{t('生成时间')}:</Text>
            <Text>{formatTime(task.created_time)}</Text>
          </div>

          {task.completed_time > 0 && (
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <Text type="tertiary">{t('完成时间')}:</Text>
              <Text>{formatTime(task.completed_time)}</Text>
            </div>
          )}
        </Space>

        {/* 图片元数据 */}
        {isSuccess && (params.size || metadata.width || metadata.height) && (
          <>
            <Divider margin="8px" />
            <div>
              <Text strong style={{ fontSize: '14px' }}>
                {t('图片元数据')}:
              </Text>
              <Space vertical spacing="loose" style={{ width: '100%', marginTop: '8px' }}>
                {params.size && (
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text type="tertiary">{t('尺寸')}:</Text>
                    <Text>{params.size}</Text>
                  </div>
                )}
                {metadata.width && (
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text type="tertiary">{t('宽度')}:</Text>
                    <Text>{metadata.width}px</Text>
                  </div>
                )}
                {metadata.height && (
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text type="tertiary">{t('高度')}:</Text>
                    <Text>{metadata.height}px</Text>
                  </div>
                )}
                {params.quality && (
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text type="tertiary">{t('质量')}:</Text>
                    <Text>{params.quality}</Text>
                  </div>
                )}
                {params.style && (
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Text type="tertiary">{t('风格')}:</Text>
                    <Text>{params.style}</Text>
                  </div>
                )}
              </Space>
            </div>
          </>
        )}

        {/* 操作按钮 */}
        <Divider margin="8px" />
        <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
          <Button onClick={onClose}>{t('关闭')}</Button>
          {isFailed && (
            <Button
              theme="solid"
              type="warning"
              icon={<IconRefresh />}
              onClick={handleRetry}
              loading={retrying}
            >
              {t('重试生成')}
            </Button>
          )}
          {isSuccess && task.image_url && (
            <Button
              theme="solid"
              type="primary"
              icon={<IconDownload />}
              onClick={handleDownload}
            >
              {t('下载图片')}
            </Button>
          )}
        </Space>
      </div>
    </Modal>
  );
};

export default ImageGenerationTaskModal;
