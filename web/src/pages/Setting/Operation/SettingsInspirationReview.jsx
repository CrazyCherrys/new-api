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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Empty,
  Form,
  Checkbox,
  Modal,
  Pagination,
  Spin,
  TabPane,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCheckCircleStroked,
  IconClose,
  IconDelete,
  IconImage,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { API, showError, showSuccess } from '../../../helpers';

const { Paragraph } = Typography;

const PAGE_SIZE = 10;

const SettingsInspirationReview = () => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('pending');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [submissions, setSubmissions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [reviewingId, setReviewingId] = useState(null);
  const [bulkProcessing, setBulkProcessing] = useState(false);
  const [selectedIds, setSelectedIds] = useState([]);
  const [rejectTarget, setRejectTarget] = useState(null);
  const [rejectReason, setRejectReason] = useState('');
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [bulkRejectOpen, setBulkRejectOpen] = useState(false);
  const [bulkRejectReason, setBulkRejectReason] = useState('');
  const currentListRef = useRef({ status: 'pending', page: 1 });
  const loadSeqRef = useRef(0);

  const statusMeta = useMemo(
    () => ({
      pending: { label: t('待审核'), color: 'orange' },
      approved: { label: t('已通过'), color: 'green' },
      rejected: { label: t('已驳回'), color: 'red' },
    }),
    [t],
  );

  useEffect(() => {
    currentListRef.current = { status, page };
  }, [status, page]);

  useEffect(() => {
    setSelectedIds([]);
  }, [status, page]);

  const loadSubmissions = useCallback(
    async (nextStatus = status, nextPage = page) => {
      const requestSeq = loadSeqRef.current + 1;
      loadSeqRef.current = requestSeq;
      setLoading(true);
      try {
        const res = await API.get('/api/image-generation/inspiration-submissions', {
          params: { status: nextStatus, p: nextPage, page_size: PAGE_SIZE },
        });
        if (requestSeq !== loadSeqRef.current) return;
        if (res.data.success) {
          const data = res.data.data || {};
          setSubmissions(data.items || []);
          setTotal(data.total || 0);
          setSelectedIds((prev) =>
            prev.filter((id) => (data.items || []).some((item) => item.id === id)),
          );
        } else {
          showError(res.data.message || t('加载灵感审核列表失败'));
        }
      } catch (error) {
        if (requestSeq !== loadSeqRef.current) return;
        showError(error.message || t('加载灵感审核列表失败'));
      } finally {
        if (requestSeq === loadSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [page, status, t],
  );

  const selectedSubmissions = useMemo(
    () => submissions.filter((item) => selectedIds.includes(item.id)),
    [selectedIds, submissions],
  );

  const hasSelection = selectedSubmissions.length > 0;
  const allCurrentSelected =
    submissions.length > 0 &&
    selectedSubmissions.length === submissions.length &&
    submissions.every((item) => selectedIds.includes(item.id));

  const toggleSelection = (itemId, checked) => {
    setSelectedIds((prev) => {
      if (checked) {
        return prev.includes(itemId) ? prev : [...prev, itemId];
      }
      return prev.filter((id) => id !== itemId);
    });
  };

  const toggleCurrentPageSelection = () => {
    if (allCurrentSelected) {
      setSelectedIds((prev) =>
        prev.filter((id) => !submissions.some((item) => item.id === id)),
      );
      return;
    }
    setSelectedIds((prev) => {
      const next = new Set(prev);
      submissions.forEach((item) => next.add(item.id));
      return Array.from(next);
    });
  };

  const clearSelection = () => {
    setSelectedIds([]);
  };

  const runBulkReview = async (targetItems, nextStatus, reason = '') => {
    if (!targetItems.length) return;
    setBulkProcessing(true);
    try {
      const results = await Promise.allSettled(
        targetItems.map((item) =>
          API.patch(
            `/api/image-generation/inspiration-submissions/${item.id}/review`,
            {
              status: nextStatus,
              reject_reason: reason,
            },
          ),
        ),
      );
      const failedCount = results.filter((res) => {
        if (res.status === 'rejected') return true;
        return !res.value?.data?.success;
      }).length;
      if (failedCount > 0) {
        showError(
          nextStatus === 'approved'
            ? t('批量通过失败')
            : t('批量驳回失败'),
        );
      } else {
        showSuccess(
          nextStatus === 'approved' ? t('批量通过成功') : t('批量驳回成功'),
        );
      }
      clearSelection();
      setBulkRejectOpen(false);
      setBulkRejectReason('');
      const currentList = currentListRef.current;
      await loadSubmissions(currentList.status, currentList.page);
    } catch (error) {
      showError(
        error.message ||
          (nextStatus === 'approved' ? t('批量通过失败') : t('批量驳回失败')),
      );
    } finally {
      setBulkProcessing(false);
    }
  };

  const runBulkDelete = async (targetItems) => {
    if (!targetItems.length) return;
    setBulkProcessing(true);
    try {
      const results = await Promise.allSettled(
        targetItems.map((item) =>
          API.delete(`/api/image-generation/inspiration-submissions/${item.id}`),
        ),
      );
      const failedCount = results.filter((res) => {
        if (res.status === 'rejected') return true;
        return !res.value?.data?.success;
      }).length;
      if (failedCount > 0) {
        showError(t('批量删除失败'));
      } else {
        showSuccess(t('批量删除成功'));
      }
      clearSelection();
      const currentList = currentListRef.current;
      await loadSubmissions(currentList.status, currentList.page);
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setBulkProcessing(false);
    }
  };

  useEffect(() => {
    loadSubmissions(status, page);
  }, [loadSubmissions, status, page]);

  const reviewSubmission = async (item, nextStatus, reason = '') => {
    setReviewingId(item.id);
    try {
      const res = await API.patch(
        `/api/image-generation/inspiration-submissions/${item.id}/review`,
        {
          status: nextStatus,
          reject_reason: reason,
        },
      );
      if (res.data.success) {
        showSuccess(nextStatus === 'approved' ? t('已通过审核') : t('已驳回'));
        setRejectTarget(null);
        setRejectReason('');
        const currentList = currentListRef.current;
        await loadSubmissions(currentList.status, currentList.page);
      } else {
        showError(res.data.message || t('审核操作失败'));
      }
    } catch (error) {
      showError(error.message || t('审核操作失败'));
    } finally {
      setReviewingId(null);
    }
  };

  const deleteSubmission = useCallback(
    async (item) => {
      setDeleteLoading(true);
      try {
        const res = await API.delete(
          `/api/image-generation/inspiration-submissions/${item.id}`,
        );
        if (res.data.success) {
          showSuccess(t('删除成功'));
          const currentList = currentListRef.current;
          await loadSubmissions(currentList.status, currentList.page);
        } else {
          showError(res.data.message || t('删除操作失败'));
        }
      } catch (error) {
        showError(error.message || t('删除操作失败'));
      } finally {
        setDeleteLoading(false);
        setDeleteTarget(null);
      }
    },
    [loadSubmissions, t],
  );

  const formatTime = (timestamp) => {
    if (!timestamp) return '-';
    return dayjs(timestamp * 1000).format('YYYY/MM/DD HH:mm');
  };

  const renderSubmission = (item) => {
    const meta = statusMeta[item.status] || statusMeta.pending;
    return (
      <div className='creative-review-item' key={item.id}>
        <div className='creative-review-preview'>
          {item.image_url ? (
            <img src={item.image_url} alt={item.prompt || 'Generated'} />
          ) : (
            <IconImage size='extra-large' />
          )}
        </div>
        <div className='creative-review-main'>
          <div className='creative-review-head'>
            <div className='creative-review-head-left'>
              <Checkbox
                checked={selectedIds.includes(item.id)}
                onChange={(e) => toggleSelection(item.id, e.target.checked)}
              />
              <div className='creative-review-title'>
                {item.display_name || item.model_id || t('未知模型')}
              </div>
            </div>
            <Tag color={meta.color}>{meta.label}</Tag>
          </div>
          <Paragraph
            copyable={{ content: item.prompt || '' }}
            ellipsis={{ rows: 2 }}
            style={{ margin: 0 }}
          >
            {item.prompt || '-'}
          </Paragraph>
          <div className='creative-review-meta'>
            <span>
              {t('提交用户')}:{' '}
              {item.user_display_name || item.username || item.user_id}
            </span>
            <span>
              {t('提交时间')}: {formatTime(item.submitted_time)}
            </span>
            {item.reviewed_time > 0 && (
              <span>
                {t('审核时间')}: {formatTime(item.reviewed_time)}
              </span>
            )}
          </div>
          {item.reject_reason && (
            <div className='creative-review-reason'>
              {t('驳回原因')}: {item.reject_reason}
            </div>
          )}
        </div>
        <div className='creative-review-actions'>
          {item.status !== 'approved' && (
            <Button
              type='primary'
              icon={<IconCheckCircleStroked />}
              loading={reviewingId === item.id}
              onClick={() => reviewSubmission(item, 'approved')}
              >
              {t('通过')}
            </Button>
          )}
          <Button
            type='tertiary'
            icon={<IconDelete />}
            loading={deleteLoading && deleteTarget?.id === item.id}
            onClick={() => setDeleteTarget(item)}
          >
            {t('删除')}
          </Button>
          {item.status !== 'rejected' && (
            <Button
              type='danger'
              icon={<IconClose />}
              loading={reviewingId === item.id}
              onClick={() => {
                setRejectTarget(item);
                setRejectReason('');
              }}
            >
              {t('驳回')}
            </Button>
          )}
        </div>
      </div>
    );
  };

  return (
    <Form.Section
      text={t('灵感审核')}
      extraText={t('审核用户提交的图片资产，通过后会公开显示在灵感')}
    >
      <style>{`
        .creative-review-toolbar {
          display: flex;
          justify-content: space-between;
          align-items: center;
          gap: 12px;
          margin-bottom: 12px;
          flex-wrap: wrap;
        }
        .creative-review-toolbar-actions {
          display: flex;
          flex-wrap: wrap;
          gap: 8px;
          align-items: center;
        }
        .creative-review-head-left {
          min-width: 0;
          display: flex;
          align-items: center;
          gap: 8px;
        }
        .creative-review-list {
          display: flex;
          flex-direction: column;
          gap: 10px;
        }
        .creative-review-item {
          display: grid;
          grid-template-columns: 112px minmax(0, 1fr) auto;
          gap: 14px;
          align-items: stretch;
          padding: 12px;
          border: 1px solid var(--semi-color-border);
          border-radius: 8px;
          background: var(--semi-color-bg-1);
        }
        .creative-review-preview {
          width: 112px;
          aspect-ratio: 1 / 1;
          border-radius: 8px;
          overflow: hidden;
          background: var(--semi-color-fill-0);
          display: flex;
          align-items: center;
          justify-content: center;
          color: var(--semi-color-text-2);
        }
        .creative-review-preview img {
          width: 100%;
          height: 100%;
          object-fit: cover;
          display: block;
        }
        .creative-review-main {
          min-width: 0;
          display: flex;
          flex-direction: column;
          gap: 8px;
        }
        .creative-review-head {
          display: flex;
          align-items: center;
          justify-content: space-between;
          gap: 8px;
        }
        .creative-review-title {
          min-width: 0;
          font-size: 14px;
          font-weight: 650;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .creative-review-meta {
          display: flex;
          flex-wrap: wrap;
          gap: 8px 14px;
          color: var(--semi-color-text-2);
          font-size: 12px;
        }
        .creative-review-reason {
          color: var(--semi-color-danger);
          font-size: 12px;
          overflow-wrap: anywhere;
        }
        .creative-review-actions {
          display: flex;
          gap: 8px;
          justify-content: center;
          min-width: 86px;
        }
        .creative-review-pagination {
          display: flex;
          justify-content: center;
          padding: 14px 0 0;
        }
        @media (max-width: 720px) {
          .creative-review-item {
            grid-template-columns: 86px minmax(0, 1fr);
          }
          .creative-review-preview {
            width: 86px;
          }
          .creative-review-actions {
            flex-direction: row;
            justify-content: flex-start;
          }
        }
      `}</style>
      <div className='creative-review-toolbar'>
        <Tabs
          type='button'
          activeKey={status}
          onChange={(key) => {
            setStatus(key);
            setPage(1);
          }}
        >
          {Object.entries(statusMeta).map(([key, meta]) => (
            <TabPane key={key} itemKey={key} tab={meta.label} />
          ))}
        </Tabs>
        <div className='creative-review-toolbar-actions'>
          {hasSelection && (
            <>
              <Tag color='blue'>
                {t('已选择 {{selected}} / {{total}}', {
                  selected: selectedSubmissions.length,
                  total: submissions.length,
                })}
              </Tag>
              <Button
                type='primary'
                loading={bulkProcessing}
                onClick={() => runBulkReview(selectedSubmissions, 'approved')}
              >
                {t('批量通过')}
              </Button>
              <Button
                type='danger'
                loading={bulkProcessing}
                onClick={() => {
                  setBulkRejectReason('');
                  setBulkRejectOpen(true);
                }}
              >
                {t('批量驳回')}
              </Button>
              <Button
                type='secondary'
                loading={bulkProcessing}
                onClick={() => {
                  Modal.confirm({
                    title: t('确认删除所选投稿？'),
                    content: t('此修改将不可逆'),
                    onOk: () => runBulkDelete(selectedSubmissions),
                  });
                }}
              >
                {t('批量删除')}
              </Button>
              <Button onClick={clearSelection}>{t('清空选择')}</Button>
            </>
          )}
          <Button
            type='tertiary'
            icon={<IconCheckCircleStroked />}
            onClick={toggleCurrentPageSelection}
          >
            {allCurrentSelected ? t('取消全选当前页') : t('全选当前页')}
          </Button>
          <Button
            icon={<IconRefresh />}
            onClick={() => loadSubmissions(status, page)}
          >
            {t('刷新')}
          </Button>
        </div>
      </div>
      <Spin spinning={loading}>
        {submissions.length > 0 ? (
          <>
            <div className='creative-review-list'>
              {submissions.map(renderSubmission)}
            </div>
            {total > PAGE_SIZE && (
              <div className='creative-review-pagination'>
                <Pagination
                  total={total}
                  currentPage={page}
                  pageSize={PAGE_SIZE}
                  onPageChange={setPage}
                />
              </div>
            )}
          </>
        ) : (
          <Empty
            image={<IconImage size='extra-large' />}
            title={t('暂无审核作品')}
          />
        )}
      </Spin>
      <Modal
        visible={Boolean(rejectTarget)}
        title={t('驳回投稿')}
        okText={t('确认驳回')}
        cancelText={t('取消')}
        onCancel={() => {
          setRejectTarget(null);
          setRejectReason('');
        }}
        onOk={() => reviewSubmission(rejectTarget, 'rejected', rejectReason)}
        confirmLoading={Boolean(
          rejectTarget && reviewingId === rejectTarget.id,
        )}
      >
        <TextArea
          value={rejectReason}
          onChange={setRejectReason}
          autosize
          placeholder={t('填写驳回原因，可选')}
        />
      </Modal>
      <Modal
        visible={bulkRejectOpen}
        title={t('批量驳回')}
        okText={t('确认驳回')}
        cancelText={t('取消')}
        onCancel={() => {
          setBulkRejectOpen(false);
          setBulkRejectReason('');
        }}
        onOk={() => runBulkReview(selectedSubmissions, 'rejected', bulkRejectReason)}
        confirmLoading={bulkProcessing}
      >
        <TextArea
          value={bulkRejectReason}
          onChange={setBulkRejectReason}
          autosize
          placeholder={t('填写驳回原因，可选')}
        />
      </Modal>
      <Modal
        visible={Boolean(deleteTarget)}
        title={t('确认删除该投稿？')}
        okText={t('确认删除')}
        cancelText={t('取消')}
        onCancel={() => setDeleteTarget(null)}
        onOk={() => deleteSubmission(deleteTarget)}
        confirmLoading={deleteLoading}
      >
        {t('此修改将不可逆')}
      </Modal>
    </Form.Section>
  );
};

export default SettingsInspirationReview;
