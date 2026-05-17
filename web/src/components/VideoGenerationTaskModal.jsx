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

import React from 'react';
import { Modal, Button, Typography, Spin } from '@douyinfe/semi-ui';
import {
  IconClose,
  IconCopy,
  IconDownload,
  IconClock,
  IconAlertTriangle,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { copy, showError, showSuccess } from '../helpers';
import { useIsMobile } from '../hooks/common/useIsMobile';

const { Text } = Typography;

const VideoGenerationTaskModal = ({ visible, onClose, task }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  if (!task) return null;

  const isSuccess = task.status === 'success';
  const isFailed = task.status === 'failed';
  const isPending = task.status === 'pending';
  const isGenerating = task.status === 'generating';
  const videoUrl = task.result_url || '';
  const posterUrl = task.thumbnail_url || '';

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    const d = new Date(timestamp * 1000);
    const pad = (n) => String(n).padStart(2, '0');
    return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(
      d.getHours(),
    )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
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

  const handleDownload = () => {
    if (!videoUrl) return;
    const link = document.createElement('a');
    link.href = videoUrl;
    link.download = `video-${task.task_id || task.id}.mp4`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const statusText = isSuccess
    ? t('生成成功')
    : isFailed
      ? t('生成失败')
      : isGenerating
        ? t('生成中')
        : t('等待中');

  const renderPreview = () => {
    if (isSuccess && videoUrl) {
      return (
        <video
          src={videoUrl}
          poster={posterUrl || undefined}
          controls
          style={{
            width: '100%',
            maxHeight: isMobile ? '40vh' : '60vh',
            borderRadius: 12,
            background: '#000',
          }}
        />
      );
    }
    if (isFailed) {
      return (
        <div
          style={{
            height: 260,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#ef4444',
            border: '1px solid rgba(239, 68, 68, 0.25)',
            borderRadius: 12,
            background: 'rgba(239, 68, 68, 0.06)',
          }}
        >
          <IconAlertTriangle size='extra-large' />
        </div>
      );
    }
    return (
      <div
        style={{
          height: 260,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 12,
          border: '1px solid var(--semi-color-border)',
          borderRadius: 12,
          background: 'var(--semi-color-fill-0)',
        }}
      >
        {isGenerating ? <Spin size='large' /> : <IconClock size='extra-large' />}
        <Text type='tertiary'>{statusText}</Text>
      </div>
    );
  };

  return (
    <Modal
      visible={visible}
      onCancel={onClose}
      footer={null}
      header={null}
      closable={false}
      centered
      width={isMobile ? 'calc(100vw - 24px)' : 960}
      bodyStyle={{
        padding: 0,
        maxHeight: isMobile ? 'calc(100vh - 24px)' : 'calc(100vh - 64px)',
        overflowY: 'auto',
      }}
      style={{ borderRadius: 12, overflow: 'hidden' }}
      maskClosable
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: isMobile ? '12px 14px' : '14px 20px',
          borderBottom: '1px solid var(--semi-color-border)',
        }}
      >
        <span style={{ fontSize: 16, fontWeight: 600 }}>
          {t('视频任务详情')}
        </span>
        <button
          type='button'
          onClick={onClose}
          style={{
            width: 28,
            height: 28,
            borderRadius: 6,
            border: 'none',
            background: 'transparent',
            cursor: 'pointer',
            color: 'var(--semi-color-text-2)',
          }}
        >
          <IconClose size='small' />
        </button>
      </div>

      <div
        style={{
          display: 'flex',
          flexDirection: isMobile ? 'column' : 'row',
          gap: 20,
          padding: isMobile ? 12 : 24,
        }}
      >
        <div style={{ flex: 1 }}>{renderPreview()}</div>

        <div
          style={{
            width: isMobile ? '100%' : 320,
            minWidth: isMobile ? 0 : 320,
            display: 'flex',
            flexDirection: 'column',
            gap: 12,
          }}
        >
          <div
            style={{
              padding: '12px 14px',
              borderRadius: 12,
              border: '1px solid var(--semi-color-border)',
              background: 'var(--semi-color-fill-0)',
            }}
          >
            <Text type='tertiary'>{t('状态')}</Text>
            <div style={{ marginTop: 6, fontWeight: 600 }}>{statusText}</div>
            {isFailed && task.error_message && (
              <div
                style={{
                  marginTop: 10,
                  padding: '10px 12px',
                  borderRadius: 8,
                  color: '#ef4444',
                  background: 'rgba(239, 68, 68, 0.08)',
                  border: '1px solid rgba(239, 68, 68, 0.25)',
                  fontSize: 12,
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                }}
              >
                {task.error_message}
              </div>
            )}
          </div>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))',
              gap: 10,
            }}
          >
            {[
              [t('模型'), task.display_name || task.model_id || '-'],
              [t('请求类型'), task.request_type || '-'],
              [t('创建时间'), formatTime(task.created_time)],
              [t('完成时间'), formatTime(task.completed_time)],
            ].map(([label, value]) => (
              <div
                key={label}
                style={{
                  padding: '10px 12px',
                  borderRadius: 12,
                  border: '1px solid var(--semi-color-border)',
                  background: 'var(--semi-color-fill-0)',
                }}
              >
                <Text type='tertiary'>{label}</Text>
                <div
                  style={{
                    marginTop: 4,
                    fontSize: 13,
                    fontWeight: 500,
                    wordBreak: 'break-all',
                  }}
                >
                  {value}
                </div>
              </div>
            ))}
          </div>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))',
              gap: 8,
            }}
          >
            <Button
              theme='outline'
              type='tertiary'
              icon={<IconCopy />}
              onClick={handleCopyPrompt}
            >
              {t('复制提示词')}
            </Button>
            <Button
              theme='outline'
              type='tertiary'
              icon={<IconDownload />}
              onClick={handleDownload}
              disabled={!isSuccess || !videoUrl}
            >
              {t('下载视频')}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default VideoGenerationTaskModal;
