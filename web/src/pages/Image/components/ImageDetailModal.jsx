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
import {
  Modal,
  Button,
  Space,
  Typography,
  Divider,
  Spin,
  Toast,
} from '@douyinfe/semi-ui';
import {
  IconDownload,
  IconDelete,
  IconRefresh,
  IconChevronLeft,
  IconChevronRight,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title, Paragraph } = Typography;

const ImageDetailModal = ({
  visible,
  image,
  onClose,
  onNavigate,
  onRegenerate,
  onDelete,
  hasNext,
  hasPrev,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [deleteModalVisible, setDeleteModalVisible] = useState(false);

  // 下载图片
  const handleDownload = async () => {
    if (!image.result_url) {
      showError(t('图片地址不存在'));
      return;
    }

    try {
      setLoading(true);
      const response = await fetch(image.result_url);
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `image_${image.id}_${Date.now()}.png`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      showSuccess(t('图片下载成功'));
    } catch (error) {
      console.error('Download error:', error);
      showError(t('图片下载失败'));
    } finally {
      setLoading(false);
    }
  };

  // 删除图片
  const handleDelete = async () => {
    try {
      setLoading(true);
      const res = await API.delete(`/api/image/task/${image.id}`);
      const { success, message } = res.data;

      if (success) {
        showSuccess(t('图片删除成功'));
        setDeleteModalVisible(false);
        onDelete();
      } else {
        showError(message || t('图片删除失败'));
      }
    } catch (error) {
      console.error('Delete error:', error);
      showError(t('图片删除失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  // 重新生成
  const handleRegenerate = () => {
    if (onRegenerate) {
      onRegenerate(image);
      onClose();
      Toast.success(t('已添加到生成队列'));
    }
  };

  // 键盘导航
  React.useEffect(() => {
    const handleKeyDown = (e) => {
      if (!visible) return;

      if (e.key === 'ArrowLeft' && hasPrev) {
        onNavigate('prev');
      } else if (e.key === 'ArrowRight' && hasNext) {
        onNavigate('next');
      } else if (e.key === 'Escape') {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [visible, hasNext, hasPrev, onNavigate, onClose]);

  const formatDate = (timestamp) => {
    if (!timestamp) return '-';
    const date = new Date(timestamp * 1000);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <>
      <Modal
        visible={visible}
        onCancel={onClose}
        footer={null}
        width={isMobile ? '95vw' : '90vw'}
        style={{ maxWidth: '1200px' }}
        bodyStyle={{ padding: 0 }}
        closeOnEsc
      >
        <Spin spinning={loading}>
          <div
            style={{
              display: 'flex',
              flexDirection: isMobile ? 'column' : 'row',
              height: isMobile ? 'auto' : '80vh',
              maxHeight: isMobile ? 'none' : '800px',
            }}
          >
            {/* 左侧：图片展示 */}
            <div
              style={{
                flex: isMobile ? 'none' : '1 1 60%',
                backgroundColor: '#000',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                position: 'relative',
                minHeight: isMobile ? '300px' : 'auto',
              }}
            >
              <img
                src={image.result_url}
                alt={image.prompt}
                style={{
                  maxWidth: '100%',
                  maxHeight: '100%',
                  objectFit: 'contain',
                }}
              />

              {/* 导航按钮 */}
              {!isMobile && (
                <>
                  {hasPrev && (
                    <Button
                      icon={<IconChevronLeft />}
                      size="large"
                      type="tertiary"
                      style={{
                        position: 'absolute',
                        left: '16px',
                        top: '50%',
                        transform: 'translateY(-50%)',
                        backgroundColor: 'rgba(0,0,0,0.5)',
                        color: 'white',
                      }}
                      onClick={() => onNavigate('prev')}
                    />
                  )}
                  {hasNext && (
                    <Button
                      icon={<IconChevronRight />}
                      size="large"
                      type="tertiary"
                      style={{
                        position: 'absolute',
                        right: '16px',
                        top: '50%',
                        transform: 'translateY(-50%)',
                        backgroundColor: 'rgba(0,0,0,0.5)',
                        color: 'white',
                      }}
                      onClick={() => onNavigate('next')}
                    />
                  )}
                </>
              )}
            </div>

            {/* 右侧：详细信息 */}
            <div
              style={{
                flex: isMobile ? 'none' : '1 1 40%',
                padding: '24px',
                overflowY: 'auto',
                backgroundColor: '#fff',
              }}
            >
              <Title heading={4} style={{ marginBottom: '16px' }}>
                {t('图片详情')}
              </Title>

              {/* 操作按钮 */}
              <Space spacing="medium" wrap style={{ marginBottom: '24px' }}>
                <Button
                  icon={<IconDownload />}
                  onClick={handleDownload}
                  disabled={loading}
                >
                  {t('下载')}
                </Button>
                <Button
                  icon={<IconRefresh />}
                  onClick={handleRegenerate}
                  disabled={loading}
                >
                  {t('重新生成')}
                </Button>
                <Button
                  icon={<IconDelete />}
                  type="danger"
                  onClick={() => setDeleteModalVisible(true)}
                  disabled={loading}
                >
                  {t('删除')}
                </Button>
              </Space>

              <Divider />

              {/* 基本信息 */}
              <div style={{ marginBottom: '16px' }}>
                <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                  {t('任务ID')}
                </Text>
                <Text type="tertiary">#{image.id}</Text>
              </div>

              <div style={{ marginBottom: '16px' }}>
                <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                  {t('模型')}
                </Text>
                <Text>{image.model}</Text>
              </div>

              <div style={{ marginBottom: '16px' }}>
                <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                  {t('提示词')}
                </Text>
                <Paragraph
                  style={{
                    backgroundColor: '#f5f5f5',
                    padding: '12px',
                    borderRadius: '4px',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                  }}
                >
                  {image.prompt}
                </Paragraph>
              </div>

              {image.resolution && (
                <div style={{ marginBottom: '16px' }}>
                  <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                    {t('分辨率')}
                  </Text>
                  <Text>{image.resolution}</Text>
                </div>
              )}

              {image.aspect_ratio && (
                <div style={{ marginBottom: '16px' }}>
                  <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                    {t('宽高比')}
                  </Text>
                  <Text>{image.aspect_ratio}</Text>
                </div>
              )}

              <div style={{ marginBottom: '16px' }}>
                <Text strong style={{ display: 'block', marginBottom: '8px' }}>
                  {t('创建时间')}
                </Text>
                <Text type="tertiary">{formatDate(image.created_at)}</Text>
              </div>

              {/* 移动端导航按钮 */}
              {isMobile && (
                <>
                  <Divider />
                  <Space spacing="medium" style={{ width: '100%' }}>
                    <Button
                      icon={<IconChevronLeft />}
                      disabled={!hasPrev}
                      onClick={() => onNavigate('prev')}
                      style={{ flex: 1 }}
                    >
                      {t('上一张')}
                    </Button>
                    <Button
                      icon={<IconChevronRight />}
                      iconPosition="right"
                      disabled={!hasNext}
                      onClick={() => onNavigate('next')}
                      style={{ flex: 1 }}
                    >
                      {t('下一张')}
                    </Button>
                  </Space>
                </>
              )}
            </div>
          </div>
        </Spin>
      </Modal>

      {/* 删除确认对话框 */}
      <Modal
        title={t('确认删除')}
        visible={deleteModalVisible}
        onOk={handleDelete}
        onCancel={() => setDeleteModalVisible(false)}
        okText={t('删除')}
        cancelText={t('取消')}
        okButtonProps={{ type: 'danger', loading }}
        cancelButtonProps={{ disabled: loading }}
      >
        {t('确定要删除这张图片吗？此操作无法撤销。')}
      </Modal>
    </>
  );
};

export default ImageDetailModal;
