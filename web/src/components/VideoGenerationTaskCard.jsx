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
      return { color: '#3ecf8e', text: t('已完成') };
    }
    if (isFailed) {
      return { color: '#ef4444', text: t('失败') };
    }
    if (isGenerating) {
      return { color: '#22d3ee', text: t('生成中') };
    }
    return { color: '#f59e0b', text: t('等待中') };
  })();

  const styles = {
    card: {
      position: 'relative',
      width: '100%',
      aspectRatio: '16 / 9',
      borderRadius: 14,
      overflow: 'hidden',
      cursor: 'pointer',
      background:
        'linear-gradient(180deg, rgba(10,18,32,0.92) 0%, rgba(24,32,48,0.96) 100%)',
      border: '1px solid var(--semi-color-border)',
      boxShadow: hovered
        ? '0 16px 32px -18px rgba(15, 23, 42, 0.48)'
        : '0 10px 22px -16px rgba(15, 23, 42, 0.36)',
      transition: 'box-shadow 0.2s ease, transform 0.2s ease',
      transform: hovered ? 'translateY(-1px)' : 'none',
    },
    poster: {
      position: 'absolute',
      inset: 0,
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      opacity: isSuccess ? 1 : 0.3,
      background: '#111827',
    },
    overlay: {
      position: 'absolute',
      inset: 0,
      background:
        'linear-gradient(to top, rgba(3,7,18,0.88), rgba(3,7,18,0.18) 55%, rgba(3,7,18,0.45))',
    },
    checkboxWrap: {
      position: 'absolute',
      top: 8,
      left: 8,
      zIndex: 6,
      padding: 4,
      borderRadius: 6,
      background: hovered || selected ? 'rgba(0,0,0,0.35)' : 'transparent',
      display: 'flex',
      alignItems: 'center',
    },
    statusBadge: {
      position: 'absolute',
      top: 8,
      right: 8,
      zIndex: 6,
      padding: '2px 8px',
      borderRadius: 999,
      fontSize: 11,
      fontWeight: 500,
      color: '#fff',
      background: statusMeta.color,
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
    footer: {
      position: 'absolute',
      left: 0,
      right: 0,
      bottom: 0,
      padding: '12px 14px',
      zIndex: 3,
      display: 'flex',
      flexDirection: 'column',
      gap: 6,
    },
    prompt: {
      color: '#fff',
      fontSize: 13,
      lineHeight: 1.35,
      maxHeight: 36,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      display: '-webkit-box',
      WebkitLineClamp: 2,
      WebkitBoxOrient: 'vertical',
    },
    metaRow: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 8,
      fontSize: 12,
      color: 'rgba(255,255,255,0.78)',
    },
    progressWrap: {
      width: '62%',
      minWidth: 180,
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
          <IconVideo size='extra-large' style={{ color: 'rgba(255,255,255,0.5)' }} />
        </div>
      )}
      <div style={styles.overlay} />

      <div style={styles.checkboxWrap} onClick={(e) => e.stopPropagation()}>
        <Checkbox
          checked={selected}
          onChange={(e) => onSelectChange(task.id, e.target.checked)}
        />
      </div>

      <span style={styles.statusBadge}>{statusMeta.text}</span>

      <div style={styles.centerWrap}>
        {isSuccess ? (
          <IconPlayCircle size='extra-large' />
        ) : isFailed ? (
          <IconAlertTriangle size='extra-large' style={{ color: statusMeta.color }} />
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
              style={{ color: 'rgba(255,255,255,0.7)', marginTop: 8, display: 'block' }}
            >
              {formatWaitTime(waitTime)}
            </Text>
          </div>
        )}
      </div>

      <div style={styles.footer}>
        <div style={styles.prompt}>{task.prompt || t('暂无提示词')}</div>
        <div style={styles.metaRow}>
          <span>{task.display_name || task.model_id || '-'}</span>
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
    prev.task.fail_reason === next.task.fail_reason
  );
});
