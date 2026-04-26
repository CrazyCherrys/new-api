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
  Typography,
  Spin,
  Progress,
  Popconfirm,
} from '@douyinfe/semi-ui';
import {
  IconDownload,
  IconRefresh,
  IconCopy,
  IconDelete,
  IconClose,
  IconImage,
  IconAlertTriangle,
  IconClock,
  IconEdit,
  IconCommentStroked,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../helpers';

const { Text } = Typography;

const ImageGenerationTaskModal = ({
  visible,
  onClose,
  task,
  onRetrySuccess,
  onDeleted,
}) => {
  const { t } = useTranslation();
  const [retrying, setRetrying] = useState(false);
  const [deleting, setDeleting] = useState(false);

  if (!task) return null;

  const isSuccess = task.status === 'success';
  const isFailed = task.status === 'failed';
  const isPending = task.status === 'pending';
  const isGenerating = task.status === 'generating';

  let metadata = {};
  try {
    if (task.image_metadata) {
      metadata = JSON.parse(task.image_metadata);
    }
  } catch (e) {
    console.error('Failed to parse image metadata:', e);
  }

  let params = {};
  try {
    if (task.params) {
      params = JSON.parse(task.params);
    }
  } catch (e) {
    console.error('Failed to parse params:', e);
  }

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    const d = new Date(timestamp * 1000);
    const pad = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(
      d.getHours(),
    )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  };

  const statusMeta = (() => {
    if (isSuccess)
      return { color: '#3ecf8e', text: t('生成成功') };
    if (isFailed)
      return { color: '#ef4444', text: t('生成失败') };
    if (isGenerating)
      return { color: '#22d3ee', text: t('生成中') };
    return { color: '#f59e0b', text: t('等待中') };
  })();

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

  const handleCopyPrompt = async () => {
    if (!task.prompt) {
      showError(t('暂无提示词'));
      return;
    }
    const ok = await copy(task.prompt);
    if (ok) {
      showSuccess(t('已复制到剪贴板'));
    } else {
      showError(t('复制失败'));
    }
  };

  const handleRetry = async () => {
    setRetrying(true);
    try {
      const response = await API.post(
        `/api/image-generation/tasks/${task.id}/retry`,
      );
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

  const handleDelete = async () => {
    setDeleting(true);
    try {
      const res = await API.delete(`/api/image-generation/tasks/${task.id}`);
      if (res.data.success) {
        showSuccess(t('删除成功'));
        if (onDeleted) onDeleted(task.id);
        onClose();
      } else {
        showError(res.data.message || t('删除失败'));
      }
    } catch (error) {
      showError(error.message || t('删除失败'));
    } finally {
      setDeleting(false);
    }
  };

  const styles = {
    headerBar: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '14px 20px',
      borderBottom: '1px solid var(--semi-color-border)',
    },
    headerTitle: {
      fontSize: 16,
      fontWeight: 600,
      color: 'var(--semi-color-text-0)',
    },
    closeBtn: {
      width: 28,
      height: 28,
      borderRadius: 6,
      border: 'none',
      background: 'transparent',
      color: 'var(--semi-color-text-2)',
      cursor: 'pointer',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition: 'background 0.15s',
    },
    body: {
      display: 'flex',
      gap: 16,
      padding: 20,
      alignItems: 'stretch',
    },
    previewCol: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      gap: 12,
    },
    previewCard: {
      position: 'relative',
      flex: 1,
      minHeight: 360,
      borderRadius: 16,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      overflow: 'hidden',
      boxShadow:
        '0 18px 40px -22px rgba(34, 211, 238, 0.45), inset 0 0 0 1px rgba(255,255,255,0.02)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    previewActions: {
      position: 'absolute',
      right: 12,
      bottom: 12,
      display: 'flex',
      gap: 8,
      zIndex: 5,
    },
    actionIconBtn: {
      width: 28,
      height: 28,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'rgba(0,0,0,0.25)',
      color: 'var(--semi-color-text-1)',
      cursor: 'pointer',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition: 'all 0.15s',
      backdropFilter: 'blur(6px)',
    },
    sideCol: {
      width: 240,
      minWidth: 240,
      display: 'flex',
      flexDirection: 'column',
      gap: 16,
    },
    infoBlock: {
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
    },
    infoLabel: {
      fontSize: 12,
      color: 'var(--semi-color-text-2)',
    },
    infoValue: {
      fontSize: 14,
      fontWeight: 500,
      color: 'var(--semi-color-text-0)',
      wordBreak: 'break-all',
    },
    errorBox: {
      marginTop: 8,
      padding: '10px 12px',
      borderRadius: 8,
      border: '1px solid rgba(239, 68, 68, 0.35)',
      background: 'rgba(239, 68, 68, 0.08)',
      color: '#ef4444',
      fontSize: 12,
      lineHeight: 1.55,
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      maxHeight: 160,
      overflowY: 'auto',
    },
    sideActions: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      marginTop: 'auto',
    },
    sideActionBtn: {
      width: '100%',
      justifyContent: 'flex-start',
    },
    statusDot: (color) => ({
      display: 'inline-block',
      width: 6,
      height: 6,
      borderRadius: '50%',
      background: color,
      marginRight: 6,
      verticalAlign: 'middle',
    }),
  };

  const renderPreview = () => {
    if (isSuccess && task.image_url) {
      return (
        <img
          src={task.image_url}
          alt='Generated'
          style={{
            maxWidth: '100%',
            maxHeight: '100%',
            objectFit: 'contain',
            display: 'block',
          }}
        />
      );
    }
    if (isFailed) {
      return (
        <div
          style={{
            width: 64,
            height: 64,
            borderRadius: '50%',
            border: '1.5px solid #ef4444',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#ef4444',
          }}
        >
          <IconAlertTriangle size='extra-large' />
        </div>
      );
    }
    if (isGenerating || isPending) {
      return (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 12,
            color: 'var(--semi-color-text-2)',
          }}
        >
          {isGenerating ? <Spin size='large' /> : <IconClock size='extra-large' />}
          <Text type='tertiary' size='small'>
            {isGenerating ? t('生成中') : t('等待中')}
          </Text>
          {typeof task.progress === 'number' && task.progress > 0 && (
            <div style={{ width: 220 }}>
              <Progress
                percent={task.progress || 0}
                showInfo
                stroke='var(--semi-color-primary)'
              />
            </div>
          )}
        </div>
      );
    }
    return (
      <div
        style={{
          color: 'var(--semi-color-text-3)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <IconImage size='extra-large' />
      </div>
    );
  };

  const renderHeader = (
    <div style={styles.headerBar}>
      <span style={styles.headerTitle}>{t('生成详情')}</span>
      <button
        type='button'
        style={styles.closeBtn}
        onClick={onClose}
        onMouseEnter={(e) =>
          (e.currentTarget.style.background = 'var(--semi-color-fill-0)')
        }
        onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
        aria-label={t('关闭')}
      >
        <IconClose size='small' />
      </button>
    </div>
  );

  const displayName =
    task.display_name || metadata.display_name || task.model_id || '-';

  // 构建尺寸文本：优先显示 aspect_ratio + resolution，回退到 size 或 width x height
  const sizeText = (() => {
    const parts = [];
    if (params.aspect_ratio) {
      parts.push(params.aspect_ratio);
    }
    if (params.resolution) {
      parts.push(params.resolution);
    }
    if (parts.length > 0) {
      return parts.join(' · ');
    }
    if (params.size) {
      return params.size;
    }
    if (metadata.width && metadata.height) {
      return `${metadata.width}x${metadata.height}`;
    }
    return '';
  })();

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      header={null}
      closable={false}
      width={1024}
      bodyStyle={{ padding: 0 }}
      style={{ borderRadius: 12, overflow: 'hidden' }}
      maskClosable
    >
      {renderHeader}

      <div style={styles.body}>
        <div style={styles.previewCol}>
          <div style={styles.previewCard}>
            {renderPreview()}

            <div style={styles.previewActions}>
              <button
                type='button'
                style={styles.actionIconBtn}
                title={t('复制提示词')}
                onClick={handleCopyPrompt}
              >
                <IconCommentStroked size='small' />
              </button>
              {isFailed && (
                <button
                  type='button'
                  style={styles.actionIconBtn}
                  title={t('重试')}
                  onClick={handleRetry}
                  disabled={retrying}
                >
                  <IconEdit size='small' />
                </button>
              )}
              {isSuccess && task.image_url && (
                <button
                  type='button'
                  style={styles.actionIconBtn}
                  title={t('下载图片')}
                  onClick={handleDownload}
                >
                  <IconDownload size='small' />
                </button>
              )}
            </div>
          </div>
        </div>

        <div style={styles.sideCol}>
          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('状态')}</span>
            <span style={{ ...styles.infoValue, color: statusMeta.color }}>
              <span style={styles.statusDot(statusMeta.color)} />
              {statusMeta.text}
            </span>
            {isFailed && task.error_message && (
              <div style={styles.errorBox}>{task.error_message}</div>
            )}
          </div>

          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('模型')}</span>
            <span style={styles.infoValue}>{displayName}</span>
          </div>

          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('创建时间')}</span>
            <span style={styles.infoValue}>{formatTime(task.created_time)}</span>
          </div>

          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('完成时间')}</span>
            <span style={styles.infoValue}>
              {formatTime(task.completed_time)}
            </span>
          </div>

          {sizeText && (
            <div style={styles.infoBlock}>
              <span style={styles.infoLabel}>{t('尺寸')}</span>
              <span style={styles.infoValue}>{sizeText}</span>
            </div>
          )}

          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('投稿状态')}</span>
            <span style={styles.infoValue}>{t('未投稿')}</span>
          </div>

          <div style={styles.infoBlock}>
            <span style={styles.infoLabel}>{t('是否公开')}</span>
            <span style={styles.infoValue}>{t('否')}</span>
          </div>

          <div style={styles.sideActions}>
            <Button
              theme='outline'
              type='tertiary'
              icon={<IconCopy />}
              style={styles.sideActionBtn}
              onClick={handleCopyPrompt}
            >
              {t('复制提示词')}
            </Button>
            <Button
              theme='outline'
              type='tertiary'
              icon={<IconDownload />}
              style={styles.sideActionBtn}
              onClick={handleDownload}
              disabled={!isSuccess || !task.image_url}
            >
              {t('下载图片')}
            </Button>
            <Button
              theme='outline'
              type='tertiary'
              icon={<IconRefresh />}
              style={styles.sideActionBtn}
              onClick={handleRetry}
              loading={retrying}
              disabled={!isFailed}
            >
              {t('重试')}
            </Button>
            <Popconfirm
              title={t('确认删除该生成记录？')}
              content={t('删除后无法恢复，请确认是否继续')}
              okText={t('确认删除')}
              cancelText={t('取消')}
              okType='danger'
              onConfirm={handleDelete}
              position='top'
            >
              <Button
                theme='outline'
                type='danger'
                icon={<IconDelete />}
                style={styles.sideActionBtn}
                loading={deleting}
              >
                {t('删除记录')}
              </Button>
            </Popconfirm>
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default ImageGenerationTaskModal;
