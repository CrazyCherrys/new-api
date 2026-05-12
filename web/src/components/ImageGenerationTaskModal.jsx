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
import { useIsMobile } from '../hooks/common/useIsMobile';

const { Text } = Typography;

const ImageGenerationTaskModal = ({
  visible,
  onClose,
  task,
  onRetrySuccess,
  onDeleted,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
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

  const parseAspectRatio = (value) => {
    if (!value) return null;
    const match = String(value)
      .trim()
      .match(/^(\d+(?:\.\d+)?)\s*(?::|x|X|\/)\s*(\d+(?:\.\d+)?)$/);
    if (!match) return null;
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!Number.isFinite(width) || !Number.isFinite(height) || height <= 0) {
      return null;
    }
    return width / height;
  };

  const metadataWidth = Number(metadata.width);
  const metadataHeight = Number(metadata.height);
  const imageAspectRatio = (() => {
    if (
      Number.isFinite(metadataWidth) &&
      metadataWidth > 0 &&
      Number.isFinite(metadataHeight) &&
      metadataHeight > 0
    ) {
      return metadataWidth / metadataHeight;
    }
    return parseAspectRatio(params.aspect_ratio) || parseAspectRatio(params.size);
  })();

  const normalizedPreviewRatio = imageAspectRatio
    ? Math.min(Math.max(imageAspectRatio, 0.56), 1.91)
    : 1;
  const isTallImage = Boolean(imageAspectRatio && imageAspectRatio < 0.8);
  const isUltraTallImage = Boolean(imageAspectRatio && imageAspectRatio < 0.65);
  const modalWidth = isMobile
    ? 'calc(100vw - 24px)'
    : 'min(1180px, calc(100vw - 64px))';
  const previewMaxWidth = isMobile
    ? '100%'
    : isUltraTallImage
      ? 360
      : isTallImage
        ? 420
        : imageAspectRatio && imageAspectRatio > 1.35
          ? '100%'
          : 640;
  const previewMaxHeight = isMobile
    ? '44vh'
    : isUltraTallImage
      ? 440
      : isTallImage
        ? 520
        : '62vh';

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    const d = new Date(timestamp * 1000);
    const pad = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(
      d.getHours(),
    )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  };

  const formatDuration = (seconds) => {
    if (!Number.isFinite(seconds) || seconds < 0) return '-';
    if (seconds < 60) return `${seconds}${t('秒')}`;
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const remainingSeconds = seconds % 60;
    if (hours > 0) {
      return `${hours}${t('小时')}${minutes}${t('分钟')}${remainingSeconds}${t('秒')}`;
    }
    return `${minutes}${t('分钟')}${remainingSeconds}${t('秒')}`;
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
      padding: isMobile ? '12px 14px' : '14px 20px',
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
      flexDirection: isMobile ? 'column' : 'row',
      gap: isMobile ? 12 : 20,
      padding: isMobile ? 12 : 24,
      alignItems: isMobile ? 'stretch' : 'flex-start',
    },
    previewCol: {
      flex: isMobile ? 'none' : isTallImage ? '0 1 460px' : '1 1 0',
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
    },
    previewCard: {
      width: '100%',
      padding: isMobile ? 12 : 16,
      borderRadius: 16,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      boxShadow:
        '0 18px 40px -22px rgba(34, 211, 238, 0.45), inset 0 0 0 1px rgba(255,255,255,0.02)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    previewFrame: {
      position: 'relative',
      width: '100%',
      maxWidth: previewMaxWidth,
      maxHeight: previewMaxHeight,
      minHeight: isMobile ? 220 : 280,
      aspectRatio: isSuccess && task.image_url ? normalizedPreviewRatio : undefined,
      borderRadius: 14,
      background: 'rgba(255,255,255,0.02)',
      overflow: 'hidden',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      margin: '0 auto',
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
      width: isMobile ? '100%' : 320,
      minWidth: isMobile ? 0 : 320,
      display: 'flex',
      flexDirection: 'column',
      gap: 12,
    },
    statusCard: {
      padding: '12px 14px',
      borderRadius: 12,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    metaGrid: {
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))',
      gap: 10,
    },
    metaCard: {
      padding: '10px 12px',
      borderRadius: 12,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
    },
    infoBlock: {
      display: 'flex',
      flexDirection: 'column',
      gap: 3,
    },
    infoLabel: {
      fontSize: 12,
      color: 'var(--semi-color-text-2)',
    },
    infoValue: {
      fontSize: 13,
      fontWeight: 500,
      color: 'var(--semi-color-text-0)',
      wordBreak: 'break-all',
      lineHeight: 1.5,
    },
    errorBox: {
      marginTop: 2,
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
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))',
      gap: 8,
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
            width: '100%',
            height: '100%',
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

  const generationDuration = (() => {
    if (!task.created_time || !task.completed_time) return '-';
    const duration = task.completed_time - task.created_time;
    return formatDuration(duration);
  })();

  const requestTypeText = (() => {
    if (task.request_type === 'edit') {
      const parts = [t('参考图编辑')];
      if (typeof task.reference_count === 'number' && task.reference_count > 0) {
        parts.push(t('参考图 {{count}} 张', { count: task.reference_count }));
      }
      if (task.has_mask) {
        parts.push(t('含遮罩'));
      }
      return parts.join(' · ');
    }
    return t('文生图');
  })();

  const outputSizeText =
    Number.isFinite(metadataWidth) &&
    metadataWidth > 0 &&
    Number.isFinite(metadataHeight) &&
    metadataHeight > 0
      ? `${metadataWidth}x${metadataHeight}`
      : '-';

  const qualityParts = [];
  if (params.quality) {
    qualityParts.push(params.quality);
  }
  if (params.style) {
    qualityParts.push(
      typeof params.style === 'string'
        ? params.style
        : JSON.stringify(params.style),
    );
  }
  if (params.n || params.quantity) {
    qualityParts.push(
      t('数量 {{count}}', {
        count: params.n || params.quantity,
      }),
    );
  }
  const qualityText = qualityParts.length > 0 ? qualityParts.join(' · ') : '-';

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
      centered
      width={modalWidth}
      bodyStyle={{
        padding: 0,
        maxHeight: isMobile ? 'calc(100vh - 24px)' : 'calc(100vh - 64px)',
        overflowY: 'auto',
        display: 'flex',
        flexDirection: 'column',
      }}
      style={{ borderRadius: isMobile ? 10 : 12, overflow: 'hidden' }}
      maskClosable
    >
      {renderHeader}

      <div style={styles.body}>
        <div style={styles.previewCol}>
          <div style={styles.previewCard}>
            <div style={styles.previewFrame}>
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
        </div>

        <div style={styles.sideCol}>
          <div style={styles.statusCard}>
            <div style={styles.infoBlock}>
              <span style={styles.infoLabel}>{t('状态')}</span>
              <span style={{ ...styles.infoValue, color: statusMeta.color }}>
                <span style={styles.statusDot(statusMeta.color)} />
                {statusMeta.text}
              </span>
            </div>
            {isFailed && task.error_message && (
              <div style={styles.errorBox}>{task.error_message}</div>
            )}
          </div>

          <div style={styles.metaGrid}>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('模型')}</span>
                <span style={styles.infoValue}>{displayName}</span>
              </div>
            </div>
            {sizeText && (
              <div style={styles.metaCard}>
                <div style={styles.infoBlock}>
                  <span style={styles.infoLabel}>{t('尺寸')}</span>
                  <span style={styles.infoValue}>{sizeText}</span>
                </div>
              </div>
            )}
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('实际输出尺寸')}</span>
                <span style={styles.infoValue}>{outputSizeText}</span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('请求类型')}</span>
                <span style={styles.infoValue}>{requestTypeText}</span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('画质参数')}</span>
                <span style={styles.infoValue}>{qualityText}</span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('消耗配额')}</span>
                <span style={styles.infoValue}>
                  {Number.isFinite(task.cost) ? task.cost : '-'}
                </span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('创建时间')}</span>
                <span style={styles.infoValue}>
                  {formatTime(task.created_time)}
                </span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('完成时间')}</span>
                <span style={styles.infoValue}>
                  {formatTime(task.completed_time)}
                </span>
              </div>
            </div>
            <div style={styles.metaCard}>
              <div style={styles.infoBlock}>
                <span style={styles.infoLabel}>{t('生成耗时')}</span>
                <span style={styles.infoValue}>{generationDuration}</span>
              </div>
            </div>
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
