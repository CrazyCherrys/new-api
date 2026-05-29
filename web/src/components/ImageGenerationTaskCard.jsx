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

import React, { useEffect, useState, memo } from 'react';
import { Checkbox, Progress, Spin, Typography } from '@douyinfe/semi-ui';
import {
  IconAlertTriangle,
  IconClock,
  IconCommentStroked,
  IconDownload,
  IconEdit,
  IconImage,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import PropTypes from 'prop-types';

const { Text } = Typography;

const ImageGenerationTaskCard = ({
  task,
  onClick,
  selected,
  onSelectChange,
}) => {
  const { t } = useTranslation();
  const [hovered, setHovered] = useState(false);
  const [waitNow, setWaitNow] = useState(() => Date.now());

  const isSuccess = task.status === 'success';
  const isFailed = task.status === 'failed';
  const isPending = task.status === 'pending';
  const isGenerating = task.status === 'generating';
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
    if (isSuccess) return { color: '#18a058', text: t('已完成') };
    if (isFailed) return { color: '#d92d20', text: t('失败') };
    if (isGenerating) return { color: '#2563eb', text: t('生成中') };
    return { color: '#b7791f', text: t('等待中') };
  })();

  const previewUrl = task.thumbnail_url || task.image_url;
  const progressValue = Number(task.progress) || 0;

  const styles = {
    card: {
      position: 'relative',
      width: '100%',
      aspectRatio: '1 / 1',
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
      transform: hovered ? 'translateY(-1px)' : 'translateY(0)',
      transition: 'border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
    },
    media: {
      position: 'absolute',
      inset: 0,
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      background: 'var(--semi-color-fill-0)',
    },
    emptyMedia: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--semi-color-text-3)',
      background: 'var(--semi-color-fill-0)',
    },
    topMask: {
      position: 'absolute',
      inset: 0,
      pointerEvents: 'none',
      background:
        'linear-gradient(180deg, rgba(0,0,0,0.28), transparent 34%, transparent 58%, rgba(0,0,0,0.72))',
    },
    checkboxWrap: {
      position: 'absolute',
      top: 8,
      left: 8,
      zIndex: 3,
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
      zIndex: 3,
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
    center: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 2,
      pointerEvents: 'none',
    },
    centerState: {
      minWidth: 148,
      maxWidth: '72%',
      padding: '12px 14px',
      borderRadius: 10,
      background: 'rgba(255,255,255,0.88)',
      border: '1px solid rgba(255,255,255,0.72)',
      boxShadow: '0 8px 24px rgba(15, 23, 42, 0.10)',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
      color: 'var(--semi-color-text-1)',
    },
    footer: {
      position: 'absolute',
      left: 0,
      right: 0,
      bottom: 0,
      zIndex: 2,
      padding: '30px 10px 10px',
      display: 'flex',
      flexDirection: 'column',
      gap: 3,
      color: '#fff',
    },
    title: {
      fontSize: 13,
      fontWeight: 600,
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textShadow: '0 1px 2px rgba(0,0,0,0.22)',
    },
    meta: {
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      minWidth: 0,
      fontSize: 12,
      color: 'rgba(255,255,255,0.78)',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    actions: {
      position: 'absolute',
      right: 8,
      bottom: 8,
      zIndex: 4,
      display: 'flex',
      gap: 6,
      opacity: hovered ? 1 : 0,
      transform: hovered ? 'translateY(0)' : 'translateY(4px)',
      pointerEvents: hovered ? 'auto' : 'none',
      transition: 'opacity 0.16s ease, transform 0.16s ease',
    },
    actionBtn: {
      width: 28,
      height: 28,
      borderRadius: 8,
      border: '1px solid rgba(255,255,255,0.24)',
      background: 'rgba(0,0,0,0.48)',
      color: '#fff',
      cursor: 'pointer',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      backdropFilter: 'blur(6px)',
    },
  };

  const renderMedia = () => {
    if (previewUrl) {
      return (
        <img
          src={previewUrl}
          alt={task.prompt || t('生成图片')}
          loading='lazy'
          decoding='async'
          draggable={false}
          style={styles.media}
        />
      );
    }
    return (
      <div style={styles.emptyMedia}>
        <IconImage size='extra-large' />
      </div>
    );
  };

  const renderCenterState = () => {
    if (isSuccess && previewUrl) {
      return null;
    }
    if (isFailed) {
      return (
        <div style={styles.center}>
          <div style={styles.centerState}>
            <IconAlertTriangle size='large' style={{ color: statusMeta.color }} />
            <Text type='tertiary' size='small' ellipsis={{ rows: 1 }}>
              {task.error_message || t('生成失败')}
            </Text>
          </div>
        </div>
      );
    }
    return (
      <div style={styles.center}>
        <div style={styles.centerState}>
          {isGenerating ? (
            <Spin size='small' />
          ) : (
            <IconClock size='large' style={{ color: statusMeta.color }} />
          )}
          <Text type='tertiary' size='small'>
            {isGenerating ? t('生成中') : t('等待中')}
          </Text>
          {isGenerating ? (
            <Progress
              percent={progressValue}
              showInfo={false}
              stroke='var(--semi-color-primary)'
              size='small'
              style={{ width: '100%' }}
            />
          ) : null}
          <Text type='tertiary' size='small' style={{ fontSize: 11 }}>
            {formatWaitTime(waitTime)}
          </Text>
        </div>
      </div>
    );
  };

  return (
    <div
      style={styles.card}
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {renderMedia()}
      <div style={styles.topMask} />
      {renderCenterState()}

      <div style={styles.checkboxWrap} onClick={(event) => event.stopPropagation()}>
        <Checkbox
          checked={selected}
          onChange={(event) => onSelectChange(task.id, event.target.checked)}
        />
      </div>

      <span style={styles.statusBadge}>
        <span style={styles.statusDot} />
        {statusMeta.text}
      </span>

      <div style={styles.footer}>
        <div style={styles.title}>{task.prompt || t('图片任务')}</div>
        <div style={styles.meta}>
          <span>{task.display_name || task.model_id || '-'}</span>
        </div>
      </div>

      <div style={styles.actions} onClick={(event) => event.stopPropagation()}>
        <button
          type='button'
          style={styles.actionBtn}
          title={t('查看详情')}
          onClick={(event) => {
            event.stopPropagation();
            onClick?.();
          }}
        >
          <IconCommentStroked size='small' />
        </button>
        {isSuccess && task.image_url ? (
          <a
            href={task.image_url}
            download={`image-${task.id}.png`}
            target='_blank'
            rel='noopener noreferrer'
            style={styles.actionBtn}
            title={t('下载图片')}
            onClick={(event) => event.stopPropagation()}
          >
            <IconDownload size='small' />
          </a>
        ) : null}
        {isFailed ? (
          <button
            type='button'
            style={styles.actionBtn}
            title={t('重试')}
            onClick={(event) => {
              event.stopPropagation();
              onClick?.();
            }}
          >
            <IconEdit size='small' />
          </button>
        ) : null}
      </div>
    </div>
  );
};

ImageGenerationTaskCard.propTypes = {
  task: PropTypes.shape({
    id: PropTypes.number.isRequired,
    status: PropTypes.oneOf(['pending', 'generating', 'success', 'failed'])
      .isRequired,
    image_url: PropTypes.string,
    thumbnail_url: PropTypes.string,
    prompt: PropTypes.string,
    progress: PropTypes.number,
    error_message: PropTypes.string,
    created_time: PropTypes.number.isRequired,
    started_time: PropTypes.number,
    display_name: PropTypes.string,
    model_id: PropTypes.string,
  }).isRequired,
  onClick: PropTypes.func,
  selected: PropTypes.bool,
  onSelectChange: PropTypes.func,
};

ImageGenerationTaskCard.defaultProps = {
  onClick: () => {},
  selected: false,
  onSelectChange: () => {},
};

export default memo(ImageGenerationTaskCard, (prev, next) => (
  prev.selected === next.selected &&
  prev.task.id === next.task.id &&
  prev.task.status === next.task.status &&
  prev.task.image_url === next.task.image_url &&
  prev.task.thumbnail_url === next.task.thumbnail_url &&
  prev.task.progress === next.task.progress &&
  prev.task.error_message === next.task.error_message
));
