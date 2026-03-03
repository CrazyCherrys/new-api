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

import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Row, Col, Select, DatePicker, Button, Empty, Spin, Toast } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import TaskCard from './TaskCard';
import { listImageTasks, deleteImageTask, createImageTask, getImageTask } from '../../helpers/imageApi';

const TaskGallery = () => {
  const { t } = useTranslation();
  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState('all');
  const [modelFilter, setModelFilter] = useState('all');
  const [dateRange, setDateRange] = useState(null);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [total, setTotal] = useState(0);

  const pollingTimersRef = useRef(new Map());

  const updateTaskInList = useCallback((taskId, updatedTask) => {
    setTasks(prevTasks =>
      prevTasks.map(task =>
        task.id === taskId ? { ...task, ...updatedTask } : task
      )
    );
  }, []);

  const pollTask = useCallback(async (taskId, attempt = 0) => {
    try {
      const response = await getImageTask(taskId);

      if (response.data?.success && response.data.data) {
        const taskData = response.data.data;
        // 标准化字段映射
        const normalizedTask = {
          ...taskData,
          image_url: taskData.image_urls?.[0],
          model: taskData.model_id
        };
        updateTaskInList(taskId, normalizedTask);

        if (taskData.status === 'pending' || taskData.status === 'running') {
          const delay = Math.min(1000 * Math.pow(2, attempt), 30000);

          const timerId = setTimeout(() => {
            pollTask(taskId, attempt + 1);
          }, delay);

          pollingTimersRef.current.set(taskId, timerId);
        } else {
          pollingTimersRef.current.delete(taskId);
        }
      }
    } catch (error) {
      console.error(`Failed to poll task ${taskId}:`, error);
      pollingTimersRef.current.delete(taskId);
    }
  }, [updateTaskInList]);

  const startPollingForTask = useCallback((taskId) => {
    if (pollingTimersRef.current.has(taskId)) {
      return;
    }
    pollTask(taskId, 0);
  }, [pollTask]);

  const stopPollingForTask = useCallback((taskId) => {
    const timerId = pollingTimersRef.current.get(taskId);
    if (timerId) {
      clearTimeout(timerId);
      pollingTimersRef.current.delete(taskId);
    }
  }, []);

  const fetchTasks = useCallback(async () => {
    try {
      setLoading(true);
      const params = {
        page,
        page_size: pageSize
      };

      if (statusFilter !== 'all') {
        params.status = statusFilter;
      }

      if (modelFilter !== 'all') {
        params.model = modelFilter;
      }

      if (dateRange && dateRange.length === 2 && dateRange[0] && dateRange[1]) {
        params.start_time = dateRange[0].toISOString();
        params.end_time = dateRange[1].toISOString();
      }

      const response = await listImageTasks(params);
      if (response.data?.success) {
        const rawTasks = response.data.data?.tasks || [];
        // 标准化字段映射：后端返回 image_urls/model_id，前端需要 image_url/model
        const fetchedTasks = rawTasks.map(task => ({
          ...task,
          image_url: task.image_urls?.[0], // 取第一张图片
          model: task.model_id // 统一字段名
        }));
        setTasks(fetchedTasks);
        setTotal(response.data.data?.total || 0);

        fetchedTasks.forEach(task => {
          if (task.status === 'pending' || task.status === 'running') {
            startPollingForTask(task.id);
          } else {
            stopPollingForTask(task.id);
          }
        });
      }
    } catch (error) {
      Toast.error(t('加载任务列表失败'));
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, statusFilter, modelFilter, dateRange, t, startPollingForTask, stopPollingForTask]);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  useEffect(() => {
    return () => {
      pollingTimersRef.current.forEach((timerId) => {
        clearTimeout(timerId);
      });
      pollingTimersRef.current.clear();
    };
  }, []);

  const handleDelete = async (taskId) => {
    try {
      stopPollingForTask(taskId);

      const response = await deleteImageTask(taskId);
      if (response.data?.success) {
        Toast.success(t('删除成功'));
        fetchTasks();
      } else {
        Toast.error(response.data?.message || t('删除失败'));
      }
    } catch (error) {
      Toast.error(t('删除失败'));
    }
  };

  const handleRegenerate = async (task) => {
    try {
      const payload = {
        prompt: task.prompt,
        ...task.params
      };
      const response = await createImageTask(payload);
      if (response.data?.success) {
        Toast.success(t('已重新提交生成任务'));

        if (response.data.data?.id) {
          startPollingForTask(response.data.data.id);
        }

        fetchTasks();
      } else {
        Toast.error(response.data?.message || t('提交失败'));
      }
    } catch (error) {
      Toast.error(t('提交失败'));
    }
  };

  const handleViewDetail = (task) => {
    console.log('View detail:', task);
  };

  const handleRefresh = () => {
    setPage(1);
    fetchTasks();
  };

  return (
    <div style={{ padding: '20px' }}>
      <div style={{ marginBottom: '20px', display: 'flex', gap: '12px', flexWrap: 'wrap', alignItems: 'center' }}>
        <Select
          value={statusFilter}
          onChange={setStatusFilter}
          style={{ width: 150 }}
          placeholder={t('状态筛选')}
        >
          <Select.Option value="all">{t('全部')}</Select.Option>
          <Select.Option value="pending">{t('等待中')}</Select.Option>
          <Select.Option value="running">{t('生成中')}</Select.Option>
          <Select.Option value="succeeded">{t('成功')}</Select.Option>
          <Select.Option value="failed">{t('失败')}</Select.Option>
        </Select>

        <Select
          value={modelFilter}
          onChange={setModelFilter}
          style={{ width: 200 }}
          placeholder={t('模型筛选')}
        >
          <Select.Option value="all">{t('全部模型')}</Select.Option>
          <Select.Option value="dall-e-2">DALL-E 2</Select.Option>
          <Select.Option value="dall-e-3">DALL-E 3</Select.Option>
          <Select.Option value="midjourney">Midjourney</Select.Option>
          <Select.Option value="stable-diffusion">Stable Diffusion</Select.Option>
        </Select>

        <DatePicker
          type="dateRange"
          value={dateRange}
          onChange={setDateRange}
          style={{ width: 300 }}
          placeholder={t('选择日期范围')}
        />

        <Button
          icon={<IconRefresh />}
          onClick={handleRefresh}
          loading={loading}
        >
          {t('刷新')}
        </Button>
      </div>

      <Spin spinning={loading}>
        {tasks.length === 0 ? (
          <Empty
            description={t('暂无任务记录')}
            style={{ padding: '60px 0' }}
          />
        ) : (
          <Row gutter={[16, 16]}>
            {tasks.map(task => (
              <Col key={task.id} xs={24} sm={12} md={8} lg={6} xl={4}>
                <TaskCard
                  task={task}
                  onDelete={handleDelete}
                  onRegenerate={handleRegenerate}
                  onViewDetail={handleViewDetail}
                />
              </Col>
            ))}
          </Row>
        )}
      </Spin>
    </div>
  );
};

export default TaskGallery;
