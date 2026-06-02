import React, { useMemo } from 'react';
import { Spin, Typography } from '@douyinfe/semi-ui';
import { IconPlayCircle, IconVideo } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { usePlayableVideo } from '../helpers/videoPlayback';

const { Text } = Typography;

const getPlaybackStatusText = (t, { hasSource, loading, error }) => {
  if (!hasSource) {
    return t('视频已生成，但暂无可播放地址');
  }
  if (loading) {
    return t('正在加载视频...');
  }
  if (error?.kind === 'auth') {
    return t('当前登录态无法直接加载该受保护视频');
  }
  if (error?.kind === 'not_found') {
    return t('视频内容不存在、已过期，或代理地址不可用');
  }
  return t('视频加载失败，请稍后重试');
};

const PlayableVideo = ({
  task,
  poster,
  style,
  onClick,
  onError,
  statusStyle,
  statusTextStyle,
  emptyIcon = (
    <IconVideo
      size='extra-large'
      style={{ color: 'var(--canvas-text-muted, var(--semi-color-text-3))' }}
    />
  ),
  videoProps = {},
}) => {
  const { t } = useTranslation();
  const { src, loading, error, hasSource, onPlaybackError } = usePlayableVideo(task);

  const statusText = useMemo(
    () => getPlaybackStatusText(t, { hasSource, loading, error }),
    [error, hasSource, loading, t],
  );

  if (!src) {
    return (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          padding: 16,
          textAlign: 'center',
          ...statusStyle,
        }}
      >
        {loading ? <Spin size='small' /> : error ? <IconPlayCircle /> : emptyIcon}
        <Text
          type={error ? 'danger' : 'tertiary'}
          size='small'
          style={{
            maxWidth: '100%',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            overflowWrap: 'anywhere',
            ...statusTextStyle,
          }}
        >
          {statusText}
        </Text>
      </div>
    );
  }

  return (
    <video
      {...videoProps}
      src={src}
      poster={poster}
      controls
      playsInline
      style={style}
      onClick={onClick}
      onError={(event) => {
        onPlaybackError();
        onError?.(event);
      }}
    />
  );
};

export default PlayableVideo;
