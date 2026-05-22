import React, { useMemo, useState } from 'react';
import { Button, Modal, Popconfirm, Typography } from '@douyinfe/semi-ui';
import {
  IconCopy,
  IconDelete,
  IconExternalOpen,
  IconRefresh,
  IconVideo,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../helpers';
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

  const styles = {
    body: {
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'minmax(0, 1.2fr) minmax(320px, 0.8fr)',
      gap: 20,
      alignItems: 'stretch',
    },
    previewPanel: {
      minHeight: isMobile ? 240 : 420,
      borderRadius: 14,
      background: '#0f172a',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    infoPanel: {
      borderRadius: 14,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      padding: 18,
      display: 'flex',
      flexDirection: 'column',
      gap: 12,
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
      borderRadius: 10,
      background: 'var(--semi-color-fill-0)',
      padding: 12,
      whiteSpace: 'pre-wrap',
      lineHeight: 1.6,
    },
    footer: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 12,
      flexWrap: 'wrap',
      marginTop: 20,
    },
    actions: {
      display: 'flex',
      gap: 8,
      flexWrap: 'wrap',
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
          {task.video_url || task.result_url ? (
            <video
              src={task.video_url || task.result_url}
              controls
              playsInline
              style={{ width: '100%', height: '100%', objectFit: 'contain' }}
            />
          ) : (
            <IconVideo size='extra-large' style={{ color: 'rgba(255,255,255,0.55)' }} />
          )}
        </div>

        <div style={styles.infoPanel}>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('状态')}</span>
            <span style={styles.value}>{statusText}</span>
          </div>
          <div style={styles.infoRow}>
            <span style={styles.label}>{t('模型')}</span>
            <span style={styles.value}>{task.display_name || task.model_id || '-'}</span>
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
      </div>

      <div style={styles.footer}>
        <div style={styles.actions}>
          <Button icon={<IconCopy />} onClick={handleCopyPrompt}>
            {t('复制提示词')}
          </Button>
          <Button icon={<IconExternalOpen />} onClick={() => onOpenTaskLogs?.(task)}>
            {t('查看任务日志')}
          </Button>
          {(task.video_url || task.result_url) && (
            <Button
              icon={<IconExternalOpen />}
              onClick={() => window.open(task.video_url || task.result_url, '_blank')}
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
