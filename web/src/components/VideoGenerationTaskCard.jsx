import React, { memo, useEffect, useState } from 'react';
import { Checkbox, Progress, Typography } from '@douyinfe/semi-ui';
import {
  IconAlertTriangle,
  IconClock,
  IconPlayCircle,
  IconVideo,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import PropTypes from 'prop-types';

const { Text } = Typography;

const VideoGenerationTaskCard = ({
  task,
  onClick,
  selected,
  onSelectChange,
}) => {
  const { t } = useTranslation();
  const [hovered, setHovered] = useState(false);
  const [waitNow, setWaitNow] = useState(() => Date.now());

  const isSuccess = task.status === 'completed';
  const isFailed = task.status === 'failed';
  const isGenerating = task.status === 'in_progress';
  const isPending = task.status === 'queued';
  const isActive = isPending || isGenerating;

  useEffect(() => {
    if (!isActive) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      setWaitNow(Date.now());
    }, 1000);
    return () => window.clearInterval(timer);
  }, [isActive]);

  const waitTime =
    isActive
      ? Math.max(
          0,
          Math.floor(
            (waitNow - (task.started_time || task.created_time) * 1000) / 1000,
          ),
        )
      : 0;

  const formatWaitTime = (seconds) => {
    if (seconds < 60) return `${seconds}${t('秒')}`;
    const minutes = Math.floor(seconds / 60);
    const remain = seconds % 60;
    return `${minutes}${t('分')}${remain}${t('秒')}`;
  };

  const statusMeta = (() => {
    if (isSuccess) {
      return { color: '#18a058', text: t('已完成') };
    }
    if (isFailed) {
      return { color: '#d92d20', text: t('失败') };
    }
    if (isGenerating) {
      return { color: '#2563eb', text: t('生成中') };
    }
    return { color: '#b7791f', text: t('等待中') };
  })();

  const styles = {
    card: {
      position: 'relative',
      width: '100%',
      aspectRatio: '16 / 9',
      borderRadius: 10,
      overflow: 'hidden',
      cursor: 'pointer',
      background: 'var(--semi-color-bg-0)',
      border: selected
        ? '1px solid var(--semi-color-primary)'
        : '1px solid var(--semi-color-border)',
      boxShadow: hovered
        ? '0 10px 24px rgba(15, 23, 42, 0.12)'
        : '0 1px 2px rgba(15, 23, 42, 0.04)',
      transition: 'border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
      transform: hovered ? 'translateY(-1px)' : 'translateY(0)',
    },
    poster: {
      position: 'absolute',
      inset: 0,
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      opacity: isSuccess ? 1 : 0.56,
      background: 'var(--semi-color-fill-0)',
    },
    overlay: {
      position: 'absolute',
      inset: 0,
      background:
        'linear-gradient(180deg, rgba(0,0,0,0.28), transparent 34%, transparent 58%, rgba(0,0,0,0.72))',
    },
    checkboxWrap: {
      position: 'absolute',
      top: 8,
      left: 8,
      zIndex: 6,
      width: 28,
      height: 28,
      borderRadius: 8,
      background: hovered || selected ? 'rgba(0,0,0,0.42)' : 'rgba(0,0,0,0.24)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      backdropFilter: 'blur(6px)',
    },
    statusBadge: {
      position: 'absolute',
      top: 8,
      right: 8,
      zIndex: 6,
      display: 'inline-flex',
      alignItems: 'center',
      gap: 5,
      maxWidth: 'calc(100% - 52px)',
      padding: '3px 8px',
      borderRadius: 999,
      background: 'rgba(0,0,0,0.48)',
      color: '#fff',
      fontSize: 11,
      fontWeight: 600,
      lineHeight: 1.2,
      backdropFilter: 'blur(6px)',
    },
    statusDot: {
      width: 6,
      height: 6,
      borderRadius: '50%',
      background: statusMeta.color,
      flexShrink: 0,
    },
    centerWrap: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 2,
      color: '#fff',
    },
    errorWrap: {
      width: '86%',
      minWidth: 148,
      padding: '12px 14px',
      borderRadius: 10,
      background: 'rgba(255,255,255,0.88)',
      border: '1px solid rgba(255,255,255,0.72)',
      boxShadow: '0 8px 24px rgba(15, 23, 42, 0.10)',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
    },
    errorText: {
      maxWidth: '100%',
      color: 'var(--semi-color-text-2)',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
      textAlign: 'center',
      lineHeight: 1.45,
    },
    footer: {
      position: 'absolute',
      left: 0,
      right: 0,
      bottom: 0,
      padding: '30px 10px 10px',
      zIndex: 3,
      display: 'flex',
      flexDirection: 'column',
      gap: 3,
    },
    prompt: {
      color: '#fff',
      fontSize: 13,
      fontWeight: 600,
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textShadow: '0 1px 2px rgba(0,0,0,0.22)',
    },
    metaRow: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 8,
      fontSize: 12,
      color: 'rgba(255,255,255,0.78)',
      overflow: 'hidden',
      whiteSpace: 'nowrap',
    },
    metaText: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
    },
    progressWrap: {
      width: '72%',
      minWidth: 148,
      padding: '12px 14px',
      borderRadius: 10,
      background: 'rgba(255,255,255,0.88)',
      border: '1px solid rgba(255,255,255,0.72)',
      boxShadow: '0 8px 24px rgba(15, 23, 42, 0.10)',
    },
  };

  return (
    <div
      style={styles.card}
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {task.thumbnail_url ? (
        <img
          src={task.thumbnail_url}
          alt={task.prompt || t('视频任务')}
          style={styles.poster}
          loading='lazy'
          decoding='async'
          draggable={false}
        />
      ) : (
        <div style={{ ...styles.poster, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <IconVideo size='extra-large' style={{ color: 'var(--semi-color-text-3)' }} />
        </div>
      )}
      <div style={styles.overlay} />

      <div style={styles.checkboxWrap} onClick={(e) => e.stopPropagation()}>
        <Checkbox
          checked={selected}
          onChange={(e) => onSelectChange(task.id, e.target.checked)}
        />
      </div>

      <span style={styles.statusBadge}>
        <span style={styles.statusDot} />
        {statusMeta.text}
      </span>

      <div style={styles.centerWrap}>
        {isSuccess ? (
          <IconPlayCircle size='extra-large' />
        ) : isFailed ? (
          <div style={styles.errorWrap}>
            <IconAlertTriangle size='large' style={{ color: statusMeta.color }} />
            <Text type='tertiary' size='small' style={styles.errorText}>
              {task.error_message || task.fail_reason || t('生成失败')}
            </Text>
          </div>
        ) : (
          <div style={{ ...styles.progressWrap, textAlign: 'center' }}>
            <IconClock size='large' style={{ marginBottom: 10, color: statusMeta.color }} />
            <Progress
              percent={parseInt(String(task.progress || '0').replace('%', ''), 10) || 0}
              showInfo={false}
              stroke={statusMeta.color}
              size='small'
            />
            <Text
              type='tertiary'
              size='small'
              style={{ color: 'var(--semi-color-text-2)', marginTop: 8, display: 'block' }}
            >
              {formatWaitTime(waitTime)}
            </Text>
          </div>
        )}
      </div>

      <div style={styles.footer}>
        <div style={styles.prompt}>{task.prompt || t('暂无提示词')}</div>
        <div style={styles.metaRow}>
          <span style={styles.metaText}>{task.display_name || task.model_id || '-'}</span>
          <span>{task.duration ? `${task.duration}s` : '-'}</span>
        </div>
      </div>
    </div>
  );
};

VideoGenerationTaskCard.propTypes = {
  task: PropTypes.shape({
    id: PropTypes.number.isRequired,
    status: PropTypes.string.isRequired,
    thumbnail_url: PropTypes.string,
    prompt: PropTypes.string,
    progress: PropTypes.string,
    error_message: PropTypes.string,
    fail_reason: PropTypes.string,
    created_time: PropTypes.number.isRequired,
    started_time: PropTypes.number,
    display_name: PropTypes.string,
    model_id: PropTypes.string,
    duration: PropTypes.number,
  }).isRequired,
  onClick: PropTypes.func,
  selected: PropTypes.bool,
  onSelectChange: PropTypes.func,
};

VideoGenerationTaskCard.defaultProps = {
  onClick: () => {},
  selected: false,
  onSelectChange: () => {},
};

export default memo(VideoGenerationTaskCard, (prev, next) => {
  return (
    prev.selected === next.selected &&
    prev.task.id === next.task.id &&
    prev.task.status === next.task.status &&
    prev.task.thumbnail_url === next.task.thumbnail_url &&
    prev.task.progress === next.task.progress &&
    prev.task.error_message === next.task.error_message &&
    prev.task.fail_reason === next.task.fail_reason
  );
});
