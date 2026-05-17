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

import React, { useState, useEffect, memo } from 'react';
import { Spin, Typography, Checkbox, Progress } from '@douyinfe/semi-ui';
import {
  IconImage,
  IconAlertTriangle,
  IconClock,
  IconCommentStroked,
  IconEdit,
  IconDownload,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import PropTypes from 'prop-types';

const { Text } = Typography;

/**
 * 图片生成任务卡片
 * - pending/generating：环形/旋转动画 + 进度 + 等待时长
 * - success：缩略图填充
 * - failed：居中红色错误图标
 * 视觉风格与「生成详情」弹窗保持一致：圆角、底部柔光、悬浮动作图标。
 */
const ImageGenerationTaskCard = ({
  task,
  onClick,
  selected,
  onSelectChange,
}) => {
  const { t } = useTranslation();
  const [hovered, setHovered] = useState(false);
  const [waitNow, setWaitNow] = useState(() => Date.now());

  const formatWaitTime = (seconds) => {
    if (seconds < 60) return `${seconds}${t('秒')}`;
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}${t('分')}${s}${t('秒')}`;
  };

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
            (waitNow - task.created_time * 1000) / 1000,
          ),
        )
      : 0;

  const statusMeta = (() => {
    if (isSuccess)
      return { color: '#3ecf8e', text: t('已完成'), glow: 'rgba(62,207,142,0.35)' };
    if (isFailed)
      return { color: '#ef4444', text: t('失败'), glow: 'rgba(239,68,68,0.35)' };
    if (isGenerating)
      return { color: '#22d3ee', text: t('生成中'), glow: 'rgba(34,211,238,0.35)' };
    return { color: '#f59e0b', text: t('等待中'), glow: 'rgba(245,158,11,0.35)' };
  })();

  const styles = {
    card: {
      position: 'relative',
      width: '100%',
      aspectRatio: '1 / 1',
      borderRadius: 14,
      overflow: 'hidden',
      cursor: 'pointer',
      background: 'var(--semi-color-fill-0)',
      border: '1px solid var(--semi-color-border)',
      boxShadow: hovered
        ? `0 14px 32px -18px ${statusMeta.glow}, 0 0 0 1px rgba(255,255,255,0.04)`
        : `0 8px 22px -16px ${statusMeta.glow}`,
      transition: 'box-shadow 0.2s ease, transform 0.2s ease',
      transform: hovered ? 'translateY(-1px)' : 'none',
    },
    body: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    image: {
      position: 'absolute',
      inset: 0,
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      backgroundColor: 'var(--semi-color-fill-1)',
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
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
      boxShadow: '0 2px 6px rgba(0,0,0,0.18)',
    },
    checkboxWrap: {
      position: 'absolute',
      top: 8,
      left: 8,
      zIndex: 6,
      padding: 4,
      borderRadius: 6,
      background: hovered || selected ? 'rgba(0,0,0,0.4)' : 'transparent',
      backdropFilter: hovered || selected ? 'blur(4px)' : 'none',
      transition: 'background 0.15s',
      display: 'flex',
      alignItems: 'center',
    },
    actions: {
      position: 'absolute',
      right: 8,
      bottom: 8,
      display: 'flex',
      gap: 6,
      zIndex: 6,
      opacity: hovered ? 1 : 0,
      transform: hovered ? 'translateY(0)' : 'translateY(4px)',
      transition: 'opacity 0.18s, transform 0.18s',
      pointerEvents: hovered ? 'auto' : 'none',
    },
    actionBtn: {
      width: 26,
      height: 26,
      borderRadius: 7,
      border: '1px solid rgba(255,255,255,0.12)',
      background: 'rgba(0,0,0,0.45)',
      color: '#fff',
      cursor: 'pointer',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      backdropFilter: 'blur(6px)',
      transition: 'background 0.15s',
    },
    centerIcon: (color) => ({
      width: 56,
      height: 56,
      borderRadius: '50%',
      border: `1.5px solid ${color}`,
      color: color,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: 'rgba(0,0,0,0.06)',
    }),
    pendingWrap: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: 8,
      padding: 12,
      width: '100%',
    },
    glowOverlay: {
      position: 'absolute',
      left: 0,
      right: 0,
      bottom: 0,
      height: 60,
      pointerEvents: 'none',
      background: `linear-gradient(to top, ${statusMeta.glow}, transparent)`,
      opacity: isSuccess ? 0 : 0.85,
    },
    overlayDarken: {
      position: 'absolute',
      inset: 0,
      pointerEvents: 'none',
      background: hovered && isSuccess
        ? 'linear-gradient(to top, rgba(0,0,0,0.45), transparent 55%)'
        : 'transparent',
      transition: 'background 0.18s',
    },
  };

  const renderCenter = () => {
    if (isSuccess) {
      const previewUrl = task.thumbnail_url || task.image_url;
      return (
        <>
          {previewUrl && (
            <img
              src={previewUrl}
              alt={task.prompt || t('生成图片')}
              loading='lazy'
              decoding='async'
              draggable={false}
              style={styles.image}
            />
          )}
          {!previewUrl && (
            <div style={styles.body}>
              <div style={{ color: 'var(--semi-color-text-3)' }}>
                <IconImage size='extra-large' />
              </div>
            </div>
          )}
        </>
      );
    }
    if (isFailed) {
      return (
        <div style={styles.body}>
          <div style={styles.centerIcon(statusMeta.color)}>
            <IconAlertTriangle size='large' />
          </div>
        </div>
      );
    }
    return (
      <div style={styles.body}>
        <div style={styles.pendingWrap}>
          {isGenerating ? (
            <Spin size='large' />
          ) : (
            <div style={styles.centerIcon(statusMeta.color)}>
              <IconClock size='large' />
            </div>
          )}
          <Text type='tertiary' size='small'>
            {isGenerating ? t('生成中') : t('等待中')}
          </Text>
          {isGenerating && (
            <div style={{ width: '80%' }}>
              <Progress
                percent={task.progress || 0}
                showInfo={false}
                stroke='var(--semi-color-primary)'
                size='small'
              />
            </div>
          )}
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
      {renderCenter()}

      <div style={styles.glowOverlay} />
      <div style={styles.overlayDarken} />

      <div
        style={styles.checkboxWrap}
        onClick={(e) => e.stopPropagation()}
      >
        <Checkbox
          checked={selected}
          onChange={(e) => onSelectChange(task.id, e.target.checked)}
        />
      </div>

      <span style={styles.statusBadge}>
        <span
          style={{
            width: 5,
            height: 5,
            borderRadius: '50%',
            background: '#fff',
            opacity: 0.85,
          }}
        />
        {statusMeta.text}
      </span>

      <div style={styles.actions} onClick={(e) => e.stopPropagation()}>
        <button
          type='button'
          style={styles.actionBtn}
          title={t('查看详情')}
          onClick={(e) => {
            e.stopPropagation();
            if (onClick) onClick();
          }}
        >
          <IconCommentStroked size='small' />
        </button>
        {isSuccess && task.image_url && (
          <a
            href={task.image_url}
            download={`image-${task.id}.png`}
            target='_blank'
            rel='noopener noreferrer'
            style={styles.actionBtn}
            title={t('下载图片')}
            onClick={(e) => e.stopPropagation()}
          >
            <IconDownload size='small' />
          </a>
        )}
        {isFailed && (
          <button
            type='button'
            style={styles.actionBtn}
            title={t('重试')}
            onClick={(e) => {
              e.stopPropagation();
              if (onClick) onClick();
            }}
          >
            <IconEdit size='small' />
          </button>
        )}
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

// 仅当影响视觉的 task 字段或 selected 发生变化时才重新渲染，
// 忽略 onClick/onSelectChange 等每次父渲染都会生成新引用的回调函数。
export default memo(ImageGenerationTaskCard, (prev, next) => {
  return (
    prev.selected === next.selected &&
    prev.task.id === next.task.id &&
    prev.task.status === next.task.status &&
    prev.task.image_url === next.task.image_url &&
    prev.task.thumbnail_url === next.task.thumbnail_url &&
    prev.task.progress === next.task.progress &&
    prev.task.error_message === next.task.error_message
  );
});
