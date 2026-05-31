import React, { useMemo, useState } from 'react';
import { Button, Modal, Popconfirm, Spin, Typography } from '@douyinfe/semi-ui';
import {
  IconCopy,
  IconDelete,
  IconExternalOpen,
  IconRefresh,
  IconVideo,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../helpers';
import { usePlayableVideo } from '../helpers/videoPlayback';
import { useIsMobile } from '../hooks/common/useIsMobile';

const { Text } = Typography;

const VideoGenerationTaskModal = ({
  visible,
  onClose,
  task,
  onRetrySuccess,
  onDeleted,
  onOpenTaskLogs,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [retrying, setRetrying] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const {
    src: playableVideoSrc,
    loading: playableVideoLoading,
    error: playableVideoError,
    hasSource: hasPlayableVideoSource,
    onPlaybackError,
    openInNewTab,
  } = usePlayableVideo(visible ? task : null);

  const canDelete = task?.status === 'completed' || task?.status === 'failed';

  const statusText = useMemo(() => {
    if (!task) return '-';
    switch (task.status) {
      case 'completed':
        return t('生成成功');
      case 'failed':
        return t('生成失败');
      case 'in_progress':
        return t('生成中');
      case 'queued':
        return t('等待中');
      default:
        return task.status;
    }
  }, [task, t]);

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    const date = new Date(timestamp * 1000);
    const pad = (n) => String(n).padStart(2, '0');
    return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(
      date.getHours(),
    )}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
  };

  const previewStatusText = useMemo(() => {
    if (playableVideoLoading) {
      return t('正在加载视频...');
    }
    if (playableVideoError?.kind === 'auth') {
      return t('当前登录态无法直接加载该受保护视频');
    }
    if (playableVideoError?.kind === 'not_found') {
      return t('视频内容不存在、已过期，或代理地址不可用');
    }
    if (playableVideoError) {
      return t('视频加载失败，请稍后重试');
    }
    if (task?.status === 'failed') {
      return task?.fail_reason || t('生成失败');
    }
    if (task?.status === 'completed') {
      return hasPlayableVideoSource
        ? t('视频加载失败，请稍后重试')
        : t('视频已生成，但暂无可播放地址');
    }
    if (task?.status === 'in_progress') {
      return t('视频生成中');
    }
    if (task?.status === 'queued') {
      return t('视频排队中');
    }
    return t('暂无视频预览');
  }, [
    hasPlayableVideoSource,
    playableVideoError,
    playableVideoLoading,
    t,
    task?.fail_reason,
    task?.status,
  ]);

  const handleOpenVideo = () => {
    if (openInNewTab()) {
      return;
    }
    showError(previewStatusText || t('暂无可打开视频'));
  };

  const handleRetry = async () => {
    if (!task?.id) return;
    setRetrying(true);
    try {
      const res = await API.post(`/api/video-generation/tasks/${task.id}/retry`);
      if (res.data.success) {
        showSuccess(t('已创建新的重试任务'));
        onRetrySuccess?.(res.data.data);
        onClose();
      } else {
        showError(res.data.message || t('重试失败'));
      }
    } catch (error) {
      showError(error.message || t('重试失败'));
    } finally {
      setRetrying(false);
    }
  };

  const handleDelete = async () => {
    if (!task?.id) return;
    setDeleting(true);
    try {
      const res = await API.delete(`/api/video-generation/tasks/${task.id}`);
      if (res.data.success) {
        showSuccess(t('删除成功'));
        onDeleted?.(task.id);
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

  const handleCopyPrompt = async () => {
    if (!task?.prompt) {
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

  if (!task) {
    return null;
  }

  const videoName = task.title || task.prompt || t('视频任务');

  const styles = {
    body: {
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'minmax(0, 1.55fr) minmax(280px, 0.65fr)',
      gap: isMobile ? 12 : 18,
      padding: isMobile ? 12 : 18,
      alignItems: 'start',
    },
    previewPanel: {
      minHeight: isMobile ? 240 : 420,
      borderRadius: 10,
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    infoPanel: {
      borderRadius: 0,
      border: 'none',
      background: 'transparent',
      padding: 0,
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
    },
    infoRow: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 12,
      fontSize: 13,
    },
    label: {
      color: 'var(--semi-color-text-2)',
    },
    value: {
      color: 'var(--semi-color-text-0)',
      textAlign: 'right',
      wordBreak: 'break-word',
    },
    promptBox: {
      borderRadius: 0,
      background: 'transparent',
      padding: 0,
      whiteSpace: 'pre-wrap',
      lineHeight: 1.6,
    },
    footer: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 12,
      flexWrap: 'wrap',
      marginTop: 0,
      paddingTop: 12,
      borderTop: '1px solid var(--semi-color-border)',
    },
    actions: {
      display: 'flex',
      gap: 8,
      flexWrap: 'wrap',
    },
    previewStatus: {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 10,
      padding: 20,
      textAlign: 'center',
    },
  };

  return (
    <Modal
      title={t('视频生成详情')}
      visible={visible}
      onCancel={onClose}
      footer={null}
      centered
      width={isMobile ? 'calc(100vw - 24px)' : 'min(1180px, calc(100vw - 64px))'}
    >
      <div style={styles.body}>
        <div style={styles.previewPanel}>
          {playableVideoSrc ? (
            <video
              src={playableVideoSrc}
              poster={task.thumbnail_url || ''}
              controls
              playsInline
              style={{ width: '100%', height: '100%', objectFit: 'contain' }}
              onError={onPlaybackError}
            />
          ) : (
            <div style={styles.previewStatus}>
              {playableVideoLoading ? (
                <Spin size='small' />
              ) : (
                <IconVideo
                  size='extra-large'
                  style={{ color: 'rgba(255,255,255,0.55)' }}
                />
              )}
              <Text
                type={playableVideoError ? 'danger' : 'tertiary'}
                style={{
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  overflowWrap: 'anywhere',
                }}
              >
                {previewStatusText}
              </Text>
            </div>
          )}
        </div>

        <div style={styles.infoPanel}>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('名称')}</span>
            <span style={styles.value}>{videoName}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>model_id</span>
            <span style={styles.value}>{task.model_id || '-'}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('时长')}</span>
            <span style={styles.value}>{task.duration ? `${task.duration}s` : '-'}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('分辨率')}</span>
            <span style={styles.value}>{task.resolution || '-'}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('比例')}</span>
            <span style={styles.value}>{task.aspect_ratio || '-'}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('消耗配额')}</span>
            <span style={styles.value}>{task.quota ?? '-'}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('任务 ID')}</span>
            <span style={styles.value}>{task.id}</span>
          </div>
          <details style={{ borderTop: '1px solid var(--semi-color-border)', paddingTop: 10 }}>
            <summary
              style={{
                cursor: 'pointer',
                color: 'var(--semi-color-text-2)',
                fontSize: 12,
                lineHeight: 1.5,
              }}
            >
              {t('更多信息')}
            </summary>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 10 }}>
              <div style={styles.infoRow}>
                <span style={styles.label}>{t('状态')}</span>
                <span style={styles.value}>{statusText}</span>
              </div>
              <div style={styles.infoRow}>
                <span style={styles.label}>{t('模型')}</span>
                <span style={styles.value}>{task.display_name || task.model_id || '-'}</span>
              </div>
              <div style={styles.infoRow}>
                <span style={styles.label}>{t('创建时间')}</span>
                <span style={styles.value}>{formatTime(task.created_time)}</span>
              </div>
              <div style={styles.infoRow}>
                <span style={styles.label}>{t('完成时间')}</span>
                <span style={styles.value}>{formatTime(task.completed_time)}</span>
              </div>
              <div>
                <Text strong>{t('提示词')}</Text>
                <div style={styles.promptBox}>{task.prompt || '-'}</div>
              </div>
              {task.fail_reason ? (
                <div>
                  <Text strong>{t('失败原因')}</Text>
                  <div style={styles.promptBox}>{task.fail_reason}</div>
                </div>
              ) : null}
            </div>
          </details>
        </div>
      </div>

      <div style={styles.footer}>
        <div style={styles.actions}>
          <Button icon={<IconCopy />} onClick={handleCopyPrompt}>
            {t('复制提示词')}
          </Button>
          <Button icon={<IconExternalOpen />} onClick={() => onOpenTaskLogs?.(task)}>
            {t('查看任务日志')}
          </Button>
          {hasPlayableVideoSource && (
            <Button
              icon={<IconExternalOpen />}
              loading={playableVideoLoading}
              disabled={!playableVideoSrc}
              onClick={handleOpenVideo}
            >
              {t('打开视频')}
            </Button>
          )}
        </div>

        <div style={styles.actions}>
          {task.status === 'failed' && (
            <Button icon={<IconRefresh />} loading={retrying} onClick={handleRetry}>
              {t('重试')}
            </Button>
          )}
          <Popconfirm
            title={t('确认删除该视频任务？')}
            onConfirm={handleDelete}
            okButtonProps={{ loading: deleting }}
            disabled={!canDelete}
          >
            <Button type='danger' icon={<IconDelete />} disabled={!canDelete}>
              {t('删除')}
            </Button>
          </Popconfirm>
        </div>
      </div>
    </Modal>
  );
};

export default VideoGenerationTaskModal;
