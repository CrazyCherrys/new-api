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

import { useEffect, useRef, useCallback } from 'react';
import { getImageTask } from '../api/image';

/**
 * 智能轮询 Hook，用于实时更新任务状态
 * @param {Array<string>} taskIds - 需要轮询的任务 ID 列表
 * @param {Function} onTaskUpdate - 任务更新回调
 * @param {Function} onTaskComplete - 任务完成回调
 * @param {Object} options - 配置选项
 * @returns {Object} - { isPolling, startPolling, stopPolling }
 */
export const useTaskPolling = (
  taskIds = [],
  onTaskUpdate,
  onTaskComplete,
  options = {}
) => {
  const {
    minInterval = 2000, // 最小轮询间隔 2s
    maxInterval = 10000, // 最大轮询间隔 10s
    enabled = true, // 是否启用轮询
  } = options;

  const timeoutRef = useRef(null);
  const intervalRef = useRef(minInterval);
  const isPollingRef = useRef(false);
  const completedTasksRef = useRef(new Set());

  // 清理函数
  const cleanup = useCallback(() => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    isPollingRef.current = false;
  }, []);

  // 获取任务状态
  const fetchTaskStatus = useCallback(
    async (taskId) => {
      try {
        const res = await getImageTask(taskId);
        if (res.success && res.data) {
          return res.data;
        }
        return null;
      } catch (error) {
        console.error(`Failed to fetch task ${taskId}:`, error);
        return null;
      }
    },
    []
  );

  // 轮询逻辑
  const poll = useCallback(async () => {
    // 检查页面是否可见
    if (document.visibilityState !== 'visible') {
      // 页面不可见时，延长轮询间隔
      timeoutRef.current = setTimeout(poll, maxInterval);
      return;
    }

    // 过滤掉已完成的任务
    const activeTasks = taskIds.filter(
      (id) => !completedTasksRef.current.has(id)
    );

    if (activeTasks.length === 0) {
      cleanup();
      return;
    }

    try {
      // 并行获取所有任务状态
      const results = await Promise.all(
        activeTasks.map((taskId) => fetchTaskStatus(taskId))
      );

      let hasRunningTask = false;

      results.forEach((task, index) => {
        if (!task) return;

        const taskId = activeTasks[index];

        // 触发更新回调
        if (onTaskUpdate) {
          onTaskUpdate(task);
        }

        // 检查任务状态
        if (task.status === 'succeeded' || task.status === 'failed') {
          completedTasksRef.current.add(taskId);
          if (onTaskComplete) {
            onTaskComplete(task);
          }
        } else if (task.status === 'running' || task.status === 'pending') {
          hasRunningTask = true;
        }
      });

      // 智能调整轮询间隔
      if (hasRunningTask) {
        // 有运行中的任务，逐渐增加间隔
        intervalRef.current = Math.min(
          intervalRef.current + 1000,
          maxInterval
        );
      } else {
        // 没有运行中的任务，重置为最小间隔
        intervalRef.current = minInterval;
      }

      // 继续轮询
      if (isPollingRef.current) {
        timeoutRef.current = setTimeout(poll, intervalRef.current);
      }
    } catch (error) {
      console.error('Polling error:', error);
      // 出错时延长间隔
      timeoutRef.current = setTimeout(poll, maxInterval);
    }
  }, [
    taskIds,
    onTaskUpdate,
    onTaskComplete,
    fetchTaskStatus,
    minInterval,
    maxInterval,
    cleanup,
  ]);

  // 启动轮询
  const startPolling = useCallback(() => {
    if (!isPollingRef.current && taskIds.length > 0) {
      isPollingRef.current = true;
      intervalRef.current = minInterval;
      completedTasksRef.current.clear();
      poll();
    }
  }, [taskIds, minInterval, poll]);

  // 停止轮询
  const stopPolling = useCallback(() => {
    cleanup();
  }, [cleanup]);

  // 监听 taskIds 变化
  useEffect(() => {
    if (enabled && taskIds.length > 0) {
      startPolling();
    } else {
      stopPolling();
    }

    return () => {
      cleanup();
    };
  }, [enabled, taskIds, startPolling, stopPolling, cleanup]);

  // 监听页面可见性变化
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible' && isPollingRef.current) {
        // 页面重新可见时，立即轮询一次
        poll();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [poll]);

  return {
    isPolling: isPollingRef.current,
    startPolling,
    stopPolling,
  };
};
