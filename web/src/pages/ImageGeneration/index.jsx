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

import React, { useState, useEffect, useRef, useMemo } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Select,
  Dropdown,
  Button,
  Upload,
  Spin,
  Typography,
  Input,
  TextArea,
  SideSheet,
  Empty,
  Checkbox,
  Tag,
  Progress,
  Popconfirm,
} from '@douyinfe/semi-ui';
import {
  IconDelete,
  IconImage,
  IconChevronDown,
  IconSend,
  IconMenu,
  IconVideo,
  IconSetting,
  IconUpload,
  IconPlus,
  IconCommentStroked,
  IconArchive,
  IconRefresh,
  IconDownload,
  IconClock,
  IconAlertTriangle,
  IconPlayCircle,
  IconExternalOpen,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import ImageGenerationTaskCard from '../../components/ImageGenerationTaskCard';
import ImageGenerationTaskModal from '../../components/ImageGenerationTaskModal';
import VideoGenerationTaskCard from '../../components/VideoGenerationTaskCard';
import VideoGenerationTaskModal from '../../components/VideoGenerationTaskModal';
import {
  getCanvasImageUiState,
  getCanvasImageSelectorVisibility,
  getReferenceImageLimit,
  modelSupportsCapability,
  modelSupportsImageEditing,
  modelSupportsMaskEditing,
} from './canvasRules';

const { Text } = Typography;

const IMAGE_CAPABILITY_GENERATION = 'image_generation';
const IMAGE_CAPABILITY_EDITING = 'image_editing';
const DEFAULT_IMAGE_CAPABILITIES = [
  IMAGE_CAPABILITY_GENERATION,
  IMAGE_CAPABILITY_EDITING,
];
const VIDEO_CAPABILITY_IMAGE_TO_VIDEO = 'image_to_video';
const VIDEO_CAPABILITY_TEXT_TO_VIDEO = 'text_to_video';
const DEFAULT_POLLING_INTERVAL_SECONDS = 5;
const DEFAULT_MAX_BATCH_TASKS = 10;
const DEFAULT_TASK_PAGE_SIZE = 21;
const TASK_PAGE_SIZE_OPTIONS = [10, 21, 50, 100];
const TASK_LIST_REQUEST_TIMEOUT_MS = 20000;
const CANVAS_PREFILL_STORAGE_KEY = 'imageGen_canvasPrefill_v1';
const DEFAULT_VIDEO_PAGE_SIZE = 20;
const CANVAS_MODE_CHAT = 'chat';
const CANVAS_MODE_IMAGE = 'image';
const CANVAS_MODE_VIDEO = 'video';
const CANVAS_MODES = [CANVAS_MODE_CHAT, CANVAS_MODE_IMAGE, CANVAS_MODE_VIDEO];
const DEFAULT_ASSET_PAGE_SIZE = 24;

const normalizeImageCapabilities = (raw) => {
  if (Array.isArray(raw)) {
    return raw;
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed;
      }
    } catch (e) {
      console.error('Failed to parse image capabilities:', e);
    }
  }
  return [...DEFAULT_IMAGE_CAPABILITIES];
};

const normalizeVideoCapabilities = (raw) => {
  if (Array.isArray(raw)) {
    return raw;
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed;
      }
    } catch (e) {
      console.error('Failed to parse video capabilities:', e);
    }
  }
  return [VIDEO_CAPABILITY_IMAGE_TO_VIDEO];
};

const modelSupportsVideoCapability = (model, capability) =>
  !!model &&
  normalizeVideoCapabilities(model.video_capabilities).includes(capability);

const taskCursorPaginationSupported = (state) =>
  !!state &&
  (state.sortBy === 'created_time' || state.sortBy === 'completed_time');

const isDefaultTaskViewState = (state) =>
  !!state &&
  state.page === 1 &&
  !state.statusFilter &&
  !state.modelFilter &&
  !state.timeFilter &&
  state.sortBy === 'created_time' &&
  state.sortOrder === 'desc';

const areTasksVisuallyEquivalent = (oldTask, newTask) =>
  !!oldTask &&
  !!newTask &&
  oldTask.id === newTask.id &&
  oldTask.status === newTask.status &&
  oldTask.image_url === newTask.image_url &&
  oldTask.thumbnail_url === newTask.thumbnail_url &&
  oldTask.completed_time === newTask.completed_time &&
  oldTask.started_time === newTask.started_time &&
  oldTask.progress === newTask.progress &&
  oldTask.error_message === newTask.error_message;

const taskIsDeletable = (task) =>
  task?.status === 'success' || task?.status === 'failed';

const mergeTaskCollections = (baseTasks, incomingTasks, maxItems) => {
  const mergedById = new Map((baseTasks || []).map((task) => [task.id, task]));
  const newFrontIds = [];

  (incomingTasks || []).forEach((task) => {
    if (!task?.id) {
      return;
    }
    const existing = mergedById.get(task.id);
    mergedById.set(task.id, existing ? { ...existing, ...task } : task);
    if (!existing) {
      newFrontIds.push(task.id);
    }
  });

  const frontIdSet = new Set(newFrontIds);
  const mergedTasks = [
    ...newFrontIds.map((id) => mergedById.get(id)),
    ...(baseTasks || [])
      .map((task) => {
        if (!task?.id || frontIdSet.has(task.id)) {
          return null;
        }
        return mergedById.get(task.id) || task;
      })
      .filter(Boolean),
  ];
  if (Number.isFinite(maxItems) && maxItems > 0) {
    return mergedTasks.slice(0, maxItems);
  }
  return mergedTasks;
};

const ImageGeneration = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const isMobile = useIsMobile();

  // LocalStorage keys
  const STORAGE_KEYS = {
    GROUP: 'imageGen_selectedGroup',
    MODE: 'canvas_generation_mode',
    CHAT_MODEL: 'canvas_chat_model',
    CHAT_TEMPERATURE: 'canvas_chat_temperature',
    CHAT_CONTEXT: 'canvas_chat_context',
    CHAT_TOOLS: 'canvas_chat_tools',
    SERIES: 'imageGen_selectedSeries',
    MODEL: 'imageGen_selectedModel',
    ASPECT_RATIO: 'imageGen_aspectRatio',
    RESOLUTION: 'imageGen_resolution',
    QUANTITY: 'imageGen_quantity',
    VIDEO_SERIES: 'videoGen_selectedSeries',
    VIDEO_MODEL: 'videoGen_selectedModel',
    VIDEO_ASPECT_RATIO: 'videoGen_aspectRatio',
    VIDEO_RESOLUTION: 'videoGen_resolution',
    VIDEO_DURATION: 'videoGen_duration',
  };

  // 从 localStorage 读取初始值的辅助函数
  const getStoredValue = (key, defaultValue) => {
    try {
      const stored = localStorage.getItem(key);
      return stored !== null ? stored : defaultValue;
    } catch (e) {
      return defaultValue;
    }
  };

  const getStoredNumber = (key, defaultValue) => {
    try {
      const stored = localStorage.getItem(key);
      if (stored !== null) {
        const num = parseInt(stored, 10);
        return isNaN(num) ? defaultValue : num;
      }
      return defaultValue;
    } catch (e) {
      return defaultValue;
    }
  };

  const [loading, setLoading] = useState(false);
  const [groupLoading, setGroupLoading] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(() =>
    getStoredValue(STORAGE_KEYS.GROUP, ''),
  );
  const [generationMode, setGenerationMode] = useState(() => {
    const storedMode = getStoredValue(STORAGE_KEYS.MODE, CANVAS_MODE_IMAGE);
    return CANVAS_MODES.includes(storedMode) ? storedMode : CANVAS_MODE_IMAGE;
  });
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);
  const [videoModels, setVideoModels] = useState([]);
  const [videoFilteredModels, setVideoFilteredModels] = useState([]);
  const [mobileTaskbarVisible, setMobileTaskbarVisible] = useState(false);
  const [composerAdvancedVisible, setComposerAdvancedVisible] = useState(false);
  const [assetLibraryVisible, setAssetLibraryVisible] = useState(false);
  const [canvasAssets, setCanvasAssets] = useState([]);
  const [canvasAssetsLoading, setCanvasAssetsLoading] = useState(false);
  const [canvasAssetsError, setCanvasAssetsError] = useState('');
  const [canvasAssetsTotal, setCanvasAssetsTotal] = useState(0);
  const [canvasAssetPage, setCanvasAssetPage] = useState(1);
  const [selectedAssetPreview, setSelectedAssetPreview] = useState(null);

  const [chatModels, setChatModels] = useState([]);
  const [chatModel, setChatModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_MODEL, ''),
  );
  const [chatPrompt, setChatPrompt] = useState('');
  const [chatTemperature, setChatTemperature] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_TEMPERATURE, '0.7'),
  );
  const [chatContext, setChatContext] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_CONTEXT, '8'),
  );
  const [chatToolsEnabled, setChatToolsEnabled] = useState(() =>
    getStoredValue(STORAGE_KEYS.CHAT_TOOLS, 'false') === 'true',
  );
  const [chatAttachments, setChatAttachments] = useState([]);
  const [selectedChatId, setSelectedChatId] = useState('local-chat-default');
  const [chatTasks, setChatTasks] = useState(() => [
    {
      id: 'local-chat-default',
      title: t('新对话'),
      summary: t('从底部输入器发送第一条消息'),
      status: t('草稿'),
      updated_at: Math.floor(Date.now() / 1000),
      messages: [],
    },
  ]);

  const [selectedSeries, setSelectedSeries] = useState(() =>
    getStoredValue(STORAGE_KEYS.SERIES, ''),
  );
  const [selectedModel, setSelectedModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.MODEL, ''),
  );
  const [selectedModelData, setSelectedModelData] = useState(null);
  const [inspiration, setInspiration] = useState('');
  const [referenceImages, setReferenceImages] = useState([]);
  const [maskImage, setMaskImage] = useState(null);
  const [aspectRatio, setAspectRatio] = useState(() =>
    getStoredValue(STORAGE_KEYS.ASPECT_RATIO, ''),
  );
  const [resolution, setResolution] = useState(() =>
    getStoredValue(STORAGE_KEYS.RESOLUTION, ''),
  );
  const [quantity, setQuantity] = useState(() =>
    getStoredNumber(STORAGE_KEYS.QUANTITY, 1),
  );
  const [generating, setGenerating] = useState(false);
  const [videoGenerating, setVideoGenerating] = useState(false);

  const [availableAspectRatios, setAvailableAspectRatios] = useState([]);
  const [availableResolutions, setAvailableResolutions] = useState([]);
  const [videoAvailableAspectRatios, setVideoAvailableAspectRatios] = useState(
    [],
  );
  const [videoAvailableResolutions, setVideoAvailableResolutions] = useState(
    [],
  );

  const [videoSelectedSeries, setVideoSelectedSeries] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_SERIES, ''),
  );
  const [videoSelectedModel, setVideoSelectedModel] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_MODEL, ''),
  );
  const [videoSelectedModelData, setVideoSelectedModelData] = useState(null);
  const [videoPrompt, setVideoPrompt] = useState('');
  const [videoReferenceImage, setVideoReferenceImage] = useState(null);
  const [videoAspectRatio, setVideoAspectRatio] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_ASPECT_RATIO, ''),
  );
  const [videoResolution, setVideoResolution] = useState(() =>
    getStoredValue(STORAGE_KEYS.VIDEO_RESOLUTION, ''),
  );
  const [videoDuration, setVideoDuration] = useState(() =>
    getStoredNumber(STORAGE_KEYS.VIDEO_DURATION, 0),
  );

  const {
    showImageAspectRatioSelector,
    showImageResolutionSelector,
  } = getCanvasImageSelectorVisibility({
    model: selectedModelData,
    aspectRatios: availableAspectRatios,
    resolutions: availableResolutions,
  });
  const showVideoAspectRatioSelector = videoAvailableAspectRatios.length > 0;
  const showVideoResolutionSelector = videoAvailableResolutions.length > 0;

  // 任务列表相关状态
  const [tasks, setTasks] = useState([]);
  const [taskTotal, setTaskTotal] = useState(0);
  const [taskPage, setTaskPage] = useState(1);
  const [taskPageSize, setTaskPageSize] = useState(DEFAULT_TASK_PAGE_SIZE);
  const [taskHasMore, setTaskHasMore] = useState(false);
  const [taskNextCursor, setTaskNextCursor] = useState('');
  const [taskStatusFilter, setTaskStatusFilter] = useState(''); // '' | 'pending' | 'generating' | 'success' | 'failed'
  const [taskModelFilter, setTaskModelFilter] = useState(''); // '' | model_id
  const [taskTimeFilter, setTaskTimeFilter] = useState(''); // '' | 'today' | 'last7d' | 'last30d' | 'thisMonth'
  const [taskSortBy, setTaskSortBy] = useState('created_time'); // 'created_time' | 'completed_time' | 'status'
  const [taskSortOrder, setTaskSortOrder] = useState('desc'); // 'desc' | 'asc'
  const [selectedTask, setSelectedTask] = useState(null);
  const [taskModalVisible, setTaskModalVisible] = useState(false);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const [taskListError, setTaskListError] = useState('');
  const [selectedTaskIds, setSelectedTaskIds] = useState(new Set());
  const [deletingTasks, setDeletingTasks] = useState(false);
  const [videoTasks, setVideoTasks] = useState([]);
  const [videoTaskTotal, setVideoTaskTotal] = useState(0);
  const [videoTaskPage, setVideoTaskPage] = useState(1);
  const [videoTaskPageSize, setVideoTaskPageSize] = useState(
    DEFAULT_VIDEO_PAGE_SIZE,
  );
  const [videoTaskStatusFilter, setVideoTaskStatusFilter] = useState('');
  const [videoTaskModelFilter, setVideoTaskModelFilter] = useState('');
  const [videoTaskTimeFilter, setVideoTaskTimeFilter] = useState('');
  const [videoLoadingTasks, setVideoLoadingTasks] = useState(false);
  const [videoTaskListError, setVideoTaskListError] = useState('');
  const [videoSelectedTask, setVideoSelectedTask] = useState(null);
  const [videoTaskModalVisible, setVideoTaskModalVisible] = useState(false);
  const [videoSelectedTaskIds, setVideoSelectedTaskIds] = useState(new Set());
  const [deletingVideoTasks, setDeletingVideoTasks] = useState(false);
  const sseRef = useRef(null);
  const pollingTimerRef = useRef(null);
  const pollingIntervalRef = useRef(DEFAULT_POLLING_INTERVAL_SECONDS);
  const taskListStateRef = useRef(null);
  const taskListRequestSeqRef = useRef(0);
  const taskDetailRequestSeqRef = useRef(0);
  const drawingModelsRequestSeqRef = useRef(0);
  const loadedModelsGroupRef = useRef('');
  const taskUpdatesCompletedSinceRef = useRef(
    Math.floor(Date.now() / 1000) - 60,
  );
  const taskCursorHistoryRef = useRef(['']);
  const pendingCanvasPrefillRef = useRef(null);
  const prefillGroupFallbackNoticeShownRef = useRef(false);
  const composerComposingRef = useRef(false);
  const [maxImageSize, setMaxImageSize] = useState(10); // MB，默认 10MB
  const [userCustomWorkerKeyEnabled, setUserCustomWorkerKeyEnabled] =
    useState(false);
  const [userCustomWorkerBaseUrlAllowed, setUserCustomWorkerBaseUrlAllowed] =
    useState(false);
  const [pollingIntervalSeconds, setPollingIntervalSeconds] = useState(
    DEFAULT_POLLING_INTERVAL_SECONDS,
  );
  const [sseConnected, setSseConnected] = useState(false);
  const [isPageVisible, setIsPageVisible] = useState(() =>
    typeof document === 'undefined' ? true : !document.hidden,
  );
  const hasActiveTasks = tasks.some(
    (task) => task.status === 'pending' || task.status === 'generating',
  );
  const hasActiveVideoTasks = videoTasks.some(
    (task) => task.status === 'queued' || task.status === 'in_progress',
  );
  const canUseTaskCursorPagination = taskCursorPaginationSupported(
    taskListStateRef.current || {
      sortBy: taskSortBy,
    },
  );
  const showsReliableTaskTotal = !canUseTaskCursorPagination;
  const hasNextTaskPage = taskHasMore;

  taskListStateRef.current = {
    page: taskPage,
    pageSize: taskPageSize,
    statusFilter: taskStatusFilter,
    modelFilter: taskModelFilter,
    timeFilter: taskTimeFilter,
    sortBy: taskSortBy,
    sortOrder: taskSortOrder,
  };
  pollingIntervalRef.current = pollingIntervalSeconds;
  useEffect(() => {
    const latestCompletedTime = tasks.reduce((latest, task) => {
      const completedAt = Number(task?.completed_time) || 0;
      return completedAt > latest ? completedAt : latest;
    }, 0);
    if (latestCompletedTime > taskUpdatesCompletedSinceRef.current) {
      taskUpdatesCompletedSinceRef.current = latestCompletedTime;
    }
  }, [tasks]);

  const enabledImageModels = useMemo(
    () =>
      models.filter(
        (model) =>
          model.status === undefined ||
          model.status === null ||
          model.status === 1,
    ),
    [models],
  );

  const enabledVideoModels = useMemo(
    () =>
      videoModels.filter(
        (model) =>
          model.status === undefined ||
          model.status === null ||
          model.status === 1,
    ),
    [videoModels],
  );

  const buildRemoteReferenceFile = (imageUrl) => {
    if (!imageUrl) {
      return null;
    }
    return {
      uid: `remote-${Date.now()}`,
      name: 'reference-image',
      status: 'done',
      url: imageUrl,
      isRemote: true,
    };
  };

  useEffect(() => {
    loadImageGenerationGroups();
    loadVideoModels();
    loadChatModels();
    loadWorkerSettings();
    connectSSE();

    return () => {
      disconnectSSE();
      stopPolling();
    };
  }, []);

  useEffect(() => {
    if (!pollingTimerRef.current) {
      return;
    }
    stopPolling();
    startPolling();
  }, [pollingIntervalSeconds]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      setIsPageVisible(!document.hidden);
    };

    handleVisibilityChange();
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);

  useEffect(() => {
    const prefillFromState = location.state?.canvasPrefill;
    if (prefillFromState) {
      pendingCanvasPrefillRef.current = prefillFromState;
      prefillGroupFallbackNoticeShownRef.current = false;
      try {
        sessionStorage.setItem(
          CANVAS_PREFILL_STORAGE_KEY,
          JSON.stringify({
            source: 'inspiration',
            payload: prefillFromState,
          }),
        );
      } catch (error) {
        console.error('Failed to persist canvas prefill state:', error);
      }
      navigate(location.pathname, { replace: true, state: null });
      return;
    }
    try {
      const storedPrefill = sessionStorage.getItem(CANVAS_PREFILL_STORAGE_KEY);
      if (storedPrefill) {
        const parsed = JSON.parse(storedPrefill);
        if (parsed?.source === 'inspiration' && parsed?.payload) {
          pendingCanvasPrefillRef.current = parsed.payload;
          prefillGroupFallbackNoticeShownRef.current = false;
        } else {
          sessionStorage.removeItem(CANVAS_PREFILL_STORAGE_KEY);
        }
      }
    } catch (error) {
      console.error('Failed to restore canvas prefill state:', error);
    }
  }, [location.pathname, location.state, navigate]);

  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const taskId = searchParams.get('task_id');
    const mode = searchParams.get('mode');
    if (!taskId) return;

    const loadTaskDetail = async () => {
      try {
        if (mode === 'video') {
          const res = await API.get(`/api/video-generation/tasks/${taskId}`);
          if (res.data.success) {
            setVideoSelectedTask(res.data.data);
            setGenerationMode(CANVAS_MODE_VIDEO);
            setVideoTaskModalVisible(false);
          } else {
            showError(res.data.message || t('加载任务详情失败'));
          }
          return;
        }
        const res = await API.get(`/api/image-generation/tasks/${taskId}`);
        if (res.data.success) {
          setSelectedTask(res.data.data);
          setGenerationMode(CANVAS_MODE_IMAGE);
          setTaskModalVisible(false);
        } else {
          showError(res.data.message || t('加载任务详情失败'));
        }
      } catch (error) {
        showError(error.message || t('加载任务详情失败'));
      }
    };

    loadTaskDetail();
  }, [location.search, t]);

  useEffect(() => {
    if (generationMode === CANVAS_MODE_VIDEO) {
      loadVideoTasks();
      return;
    }
    if (generationMode === CANVAS_MODE_IMAGE) {
      loadTasks();
    }
  }, [
    generationMode,
    taskPage,
    taskPageSize,
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
    videoTaskPage,
    videoTaskPageSize,
    videoTaskStatusFilter,
    videoTaskModelFilter,
    videoTaskTimeFilter,
  ]);

  // 切换任意筛选/排序时回到第一页
  useEffect(() => {
    setTaskPage(1);
    setTaskHasMore(false);
    setTaskNextCursor('');
    taskCursorHistoryRef.current = [''];
  }, [
    taskStatusFilter,
    taskModelFilter,
    taskTimeFilter,
    taskSortBy,
    taskSortOrder,
  ]);

  useEffect(() => {
    setVideoTaskPage(1);
  }, [videoTaskStatusFilter, videoTaskModelFilter, videoTaskTimeFilter]);

  const loadImageGenerationGroups = async () => {
    setGroupLoading(true);
    try {
      const res = await API.get('/api/image-generation/groups');
      if (res.data.success) {
        const payload = res.data.data || {};
        const items = Array.isArray(payload.items) ? payload.items : [];
        const defaultGroup = payload.default_group || '';
        setGroupOptions(items);
        const availableGroups = items
          .filter((item) => item.group && item.has_available_token !== false)
          .map((item) => item.group);
        let nextGroup = getStoredValue(STORAGE_KEYS.GROUP, '');
        if (!nextGroup || !availableGroups.includes(nextGroup)) {
          const explicitDefault = items.find(
            (item) => item.is_default && item.has_available_token !== false,
          )?.group;
          nextGroup =
            explicitDefault ||
            (availableGroups.includes(defaultGroup) ? defaultGroup : '') ||
            availableGroups[0] ||
            '';
        }
        setSelectedGroup(nextGroup);
      } else {
        showError(res.data.message || t('加载分组失败'));
      }
    } catch (error) {
      showError(error.message || t('加载分组失败'));
    } finally {
      setGroupLoading(false);
    }
  };

  const loadDrawingModels = async (group) => {
    drawingModelsRequestSeqRef.current += 1;
    const requestSeq = drawingModelsRequestSeqRef.current;
    setLoading(true);
    try {
      const res = await API.get('/api/image-generation/models', {
        params: group ? { group } : undefined,
      });
      if (requestSeq !== drawingModelsRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        const drawingModels = (res.data.data || []).map((model) => ({
          ...model,
          image_capabilities: normalizeImageCapabilities(
            model.image_capabilities,
          ),
        }));
        loadedModelsGroupRef.current = group || '';
        setModels(drawingModels);

        const seriesList = Array.from(
          new Set(drawingModels.map((model) => model.model_series).filter(Boolean)),
        );
        setSelectedSeries((prev) => {
          if (prev === 'all' || (prev && seriesList.includes(prev))) {
            return prev;
          }
          return seriesList[0] || 'all';
        });
      } else {
        showError(res.data.message || t('加载模型失败'));
      }
    } catch (error) {
      if (requestSeq !== drawingModelsRequestSeqRef.current) {
        return;
      }
      showError(error.message || t('加载模型失败'));
    } finally {
      if (requestSeq === drawingModelsRequestSeqRef.current) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    if (!selectedGroup) {
      return;
    }
    loadDrawingModels(selectedGroup);
  }, [selectedGroup]);

  const loadVideoModels = async () => {
    try {
      const res = await API.get('/api/video-generation/models');
      if (!res.data.success) {
        showError(res.data.message || t('加载视频模型失败'));
        return;
      }
      const items = res.data.data || [];
      setVideoModels(
        items.map((item) => ({
          ...item,
          video_capabilities: normalizeVideoCapabilities(
            item.video_capabilities,
          ),
        })),
      );
      const series = Array.from(
        new Set(items.map((item) => item.model_series).filter(Boolean)),
      );
      setVideoSelectedSeries((prev) => {
        if (prev === 'all' || (prev && series.includes(prev))) {
          return prev;
        }
        return series[0] || 'all';
      });
    } catch (error) {
      showError(error.message || t('加载视频模型失败'));
    }
  };

  const loadChatModels = async () => {
    try {
      const res = await API.get('/api/user/models');
      if (!res.data.success) {
        showError(res.data.message || t('加载聊天模型失败'));
        return;
      }
      const items = Array.isArray(res.data.data) ? res.data.data : [];
      setChatModels(items);
      setChatModel((current) => {
        if (current && items.includes(current)) {
          return current;
        }
        return items[0] || '';
      });
    } catch (error) {
      showError(error.message || t('加载聊天模型失败'));
    }
  };

  const loadCanvasAssets = async (nextPage = 1) => {
    setCanvasAssetsLoading(true);
    setCanvasAssetsError('');
    try {
      const res = await API.get('/api/image-generation/assets', {
        params: {
          p: nextPage,
          page_size: DEFAULT_ASSET_PAGE_SIZE,
          sort_by: 'created_time',
          sort_order: 'desc',
        },
      });
      if (!res.data.success) {
        const message = res.data.message || t('加载资产失败');
        setCanvasAssetsError(message);
        showError(message);
        return;
      }
      const data = res.data.data || {};
      setCanvasAssets(data.items || []);
      setCanvasAssetsTotal(data.total || 0);
      setCanvasAssetPage(data.page || nextPage);
    } catch (error) {
      const message = error.message || t('加载资产失败');
      setCanvasAssetsError(message);
      showError(message);
    } finally {
      setCanvasAssetsLoading(false);
    }
  };

  useEffect(() => {
    if (!assetLibraryVisible) {
      return;
    }
    loadCanvasAssets(canvasAssetPage || 1);
  }, [assetLibraryVisible]);

  useEffect(() => {
    if (!videoSelectedModelData) {
      return;
    }
    if (
      !modelSupportsVideoCapability(
        videoSelectedModelData,
        VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
      )
    ) {
      setVideoReferenceImage(null);
    }
  }, [videoSelectedModelData]);

  const formatModelSeries = (series) => {
    if (!series) return '';

    const seriesMap = {
      openai: 'OpenAI',
      gemini: 'Gemini',
      claude: 'Claude',
      grok: 'Grok',
      deepseek: 'DeepSeek',
      qwen: 'Qwen',
      glm: 'GLM',
      hunyuan: 'Hunyuan',
      doubao: 'Doubao',
      spark: 'Spark',
      baichuan: 'Baichuan',
      minimax: 'Minimax',
      moonshot: 'Moonshot',
      yi: 'Yi',
      chatglm: 'ChatGLM',
      ernie: 'ERNIE',
      wenxin: 'Wenxin',
      tongyi: 'Tongyi',
      azure: 'Azure',
      aws: 'AWS',
      cohere: 'Cohere',
      anthropic: 'Anthropic',
      mistral: 'Mistral',
      llama: 'Llama',
      palm: 'PaLM',
      bard: 'Bard',
      midjourney: 'Midjourney',
      dalle: 'OpenAI',
      'stable-diffusion': 'Stable Diffusion',
      flux: 'Flux',
      suno: 'Suno',
    };

    return (
      seriesMap[series.toLowerCase()] ||
      series.charAt(0).toUpperCase() + series.slice(1)
    );
  };

  const getModelDisplayName = (model) =>
    model?.display_name || model?.request_model || '';

  const selectImageModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode(CANVAS_MODE_IMAGE);
    if (model.model_series) {
      setSelectedSeries(model.model_series);
    }
    setSelectedModel(model.request_model);
  };

  const selectVideoModelFromCatalog = (model) => {
    if (!model?.request_model) {
      return;
    }
    setGenerationMode(CANVAS_MODE_VIDEO);
    if (model.model_series) {
      setVideoSelectedSeries(model.model_series);
    }
    setVideoSelectedModel(model.request_model);
  };

  const loadWorkerSettings = async () => {
    try {
      const res = await API.get('/api/image-generation/settings');
      if (res.data.success && res.data.data) {
        const size = parseInt(res.data.data.max_image_size, 10);
        if (!isNaN(size) && size > 0) {
          setMaxImageSize(size);
        }
        const pollingInterval = parseInt(res.data.data.polling_interval, 10);
        if (!isNaN(pollingInterval) && pollingInterval > 0) {
          setPollingIntervalSeconds(pollingInterval);
        } else {
          setPollingIntervalSeconds(DEFAULT_POLLING_INTERVAL_SECONDS);
        }
        setUserCustomWorkerKeyEnabled(
          res.data.data.user_custom_key_enabled === true,
        );
        setUserCustomWorkerBaseUrlAllowed(
          res.data.data.user_custom_base_url_allowed === true,
        );
      }
    } catch (error) {
      // 静默失败，使用默认值
      console.error('Failed to load worker settings:', error);
    }
  };

  // 根据时间范围预设计算 start/end 时间戳（秒）
  const computeTimeRange = (preset) => {
    if (!preset) return { start: 0, end: 0 };
    const now = new Date();
    const end = Math.floor(now.getTime() / 1000);
    let startDate;
    switch (preset) {
      case 'today':
        startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        break;
      case 'last7d':
        startDate = new Date(now.getTime() - 7 * 24 * 3600 * 1000);
        break;
      case 'last30d':
        startDate = new Date(now.getTime() - 30 * 24 * 3600 * 1000);
        break;
      case 'thisMonth':
        startDate = new Date(now.getFullYear(), now.getMonth(), 1);
        break;
      default:
        return { start: 0, end: 0 };
    }
    return { start: Math.floor(startDate.getTime() / 1000), end };
  };

  // silent=true 时静默刷新（轮询），不会触发全局阻塞遮罩。
  // disableDuplicate=true 用于强制绕过 API 层的 in-flight GET 去重。
  const loadTasks = async (silent = false, options = {}) => {
    const { disableDuplicate = false, forceRefresh = false } = options;
    const queryState = taskListStateRef.current || {
      page: taskPage,
      pageSize: taskPageSize,
      statusFilter: taskStatusFilter,
      modelFilter: taskModelFilter,
      timeFilter: taskTimeFilter,
      sortBy: taskSortBy,
      sortOrder: taskSortOrder,
    };
    const useCursorPagination = taskCursorPaginationSupported(queryState);
    const shouldAdvanceSeq = !silent || forceRefresh;
    const requestSeq = shouldAdvanceSeq
      ? taskListRequestSeqRef.current + 1
      : taskListRequestSeqRef.current;
    if (shouldAdvanceSeq) {
      taskListRequestSeqRef.current = requestSeq;
    }
    if (!silent) setLoadingTasks(true);
    try {
      const params = {
        p: queryState.page,
        page_size: queryState.pageSize,
      };
      if (useCursorPagination) {
        const cursorHistory = taskCursorHistoryRef.current;
        params.cursor = cursorHistory[queryState.page - 1] || '';
      }
      if (queryState.statusFilter) {
        params.status = queryState.statusFilter;
      }
      if (queryState.modelFilter) {
        params.model_id = queryState.modelFilter;
      }
      const { start, end } = computeTimeRange(queryState.timeFilter);
      if (start > 0) {
        params.start_time = start;
        params.end_time = end;
      }
      if (queryState.sortBy) {
        params.sort_by = queryState.sortBy;
      }
      if (queryState.sortOrder) {
        params.sort_order = queryState.sortOrder;
      }

      const res = await API.get('/api/image-generation/tasks', {
        params,
        timeout: TASK_LIST_REQUEST_TIMEOUT_MS,
        skipErrorHandler: true,
        disableDuplicate,
      });
      if (requestSeq !== taskListRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        const newItems = res.data.data.items || [];
        const hasTotal = Number.isFinite(res.data.data.total);
        const newTotal = hasTotal ? res.data.data.total : taskTotal;
        setTaskListError('');
        // 智能合并：仅当内容实际变化时才更新，避免全量替换导致卡片无效重渲染
        setTasks((prev) => {
          if (prev.length === newItems.length) {
            const unchanged = newItems.every((newTask, i) => {
              const old = prev[i];
              return areTasksVisuallyEquivalent(old, newTask);
            });
            if (unchanged) return prev;
          }
          return newItems;
        });
        if (hasTotal) {
          setTaskTotal(newTotal);
        }
        if (useCursorPagination) {
          const nextCursor = res.data.data.next_cursor || '';
          const cursorHistory = taskCursorHistoryRef.current.slice(
            0,
            queryState.page,
          );
          if (nextCursor) {
            cursorHistory[queryState.page] = nextCursor;
          }
          taskCursorHistoryRef.current = cursorHistory;
          setTaskNextCursor(nextCursor);
        } else {
          setTaskNextCursor('');
        }
        setTaskHasMore(res.data.data.has_more === true);
        setSelectedTask((prev) => {
          if (prev && newItems.some((task) => task.id === prev.id)) {
            return {
              ...prev,
              ...newItems.find((task) => task.id === prev.id),
            };
          }
          return prev || newItems[0] || null;
        });
      } else if (!silent) {
        const message = res.data.message || t('加载任务列表失败');
        setTaskListError(message);
        showError(message);
      }
    } catch (error) {
      if (requestSeq === taskListRequestSeqRef.current && !silent) {
        const message = error.message || t('加载任务列表失败');
        setTaskListError(message);
        showError(message);
      }
    } finally {
      if (!silent && requestSeq === taskListRequestSeqRef.current) {
        setLoadingTasks(false);
      }
    }
  };

  const loadVideoTasks = async () => {
    setVideoLoadingTasks(true);
    try {
      const params = {
        p: videoTaskPage,
        page_size: videoTaskPageSize,
      };
      if (videoTaskStatusFilter) {
        params.status = videoTaskStatusFilter;
      }
      if (videoTaskModelFilter) {
        params.model_id = videoTaskModelFilter;
      }
      const { start, end } = computeTimeRange(videoTaskTimeFilter);
      if (start > 0) {
        params.start_time = start;
        params.end_time = end;
      }
      const res = await API.get('/api/video-generation/tasks', { params });
      if (res.data.success) {
        const newItems = res.data.data.items || [];
        setVideoTaskListError('');
        setVideoTasks(newItems);
        setVideoTaskTotal(res.data.data.total || 0);
        setVideoSelectedTask((prev) => {
          if (prev && newItems.some((task) => task.id === prev.id)) {
            return {
              ...prev,
              ...newItems.find((task) => task.id === prev.id),
            };
          }
          return prev || newItems[0] || null;
        });
      } else {
        const message = res.data.message || t('加载视频任务列表失败');
        setVideoTaskListError(message);
        showError(message);
      }
    } catch (error) {
      const message = error.message || t('加载视频任务列表失败');
      setVideoTaskListError(message);
      showError(message);
    } finally {
      setVideoLoadingTasks(false);
    }
  };

  const mergeTaskUpdates = (updates) => {
    if (!Array.isArray(updates) || updates.length === 0) {
      return;
    }
    setTasks((prevTasks) =>
      mergeTaskCollections(prevTasks, updates, taskPageSize),
    );
  };

  const loadTaskUpdates = async () => {
    if (!isDefaultTaskViewState(taskListStateRef.current)) {
      return;
    }
    const completedSince = Math.max(
      1,
      Number(taskUpdatesCompletedSinceRef.current) || 0,
    );

    try {
      const res = await API.get('/api/image-generation/tasks/updates', {
        params: {
          completed_since: completedSince,
          limit: Math.max(
            (taskListStateRef.current?.pageSize || taskPageSize) * 2,
            50,
          ),
        },
        timeout: TASK_LIST_REQUEST_TIMEOUT_MS,
        skipErrorHandler: true,
      });
      if (!res.data.success) {
        return;
      }
      const latestCompletedTime = (res.data.data?.items || []).reduce(
        (latest, task) => {
          const completedAt = Number(task?.completed_time) || 0;
          return completedAt > latest ? completedAt : latest;
        },
        completedSince,
      );
      if (latestCompletedTime > taskUpdatesCompletedSinceRef.current) {
        taskUpdatesCompletedSinceRef.current = latestCompletedTime;
      }
      mergeTaskUpdates(res.data.data?.items || []);
    } catch (error) {
      console.error('Failed to load task updates:', error);
    }
  };

  const handleTaskCardClick = async (task) => {
    if (!task?.id) return;

    taskDetailRequestSeqRef.current += 1;
    const requestSeq = taskDetailRequestSeqRef.current;

    setSelectedTask(task);
    setTaskModalVisible(false);

    try {
      const res = await API.get(`/api/image-generation/tasks/${task.id}`);
      if (requestSeq !== taskDetailRequestSeqRef.current) {
        return;
      }
      if (res.data.success) {
        updateTaskInList(res.data.data);
        setSelectedTask(res.data.data);
      } else {
        showError(res.data.message || t('加载任务详情失败'));
      }
    } catch (error) {
      if (requestSeq !== taskDetailRequestSeqRef.current) {
        return;
      }
      showError(error.message || t('加载任务详情失败'));
    }
  };

  const handleVideoTaskCardClick = async (task) => {
    if (!task?.id) return;
    setVideoSelectedTask(task);
    setVideoTaskModalVisible(false);
    try {
      const res = await API.get(`/api/video-generation/tasks/${task.id}`);
      if (res.data.success) {
        updateVideoTaskInList(res.data.data);
        setVideoSelectedTask(res.data.data);
      } else {
        showError(res.data.message || t('加载视频任务详情失败'));
      }
    } catch (error) {
      showError(error.message || t('加载视频任务详情失败'));
    }
  };

  // 处理任务选择
  const handleTaskSelect = (taskId, checked) => {
    setSelectedTaskIds((prev) => {
      const newSet = new Set(prev);
      if (checked) {
        newSet.add(taskId);
      } else {
        newSet.delete(taskId);
      }
      return newSet;
    });
  };

  const handleVideoTaskSelect = (taskId, checked) => {
    setVideoSelectedTaskIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(taskId);
      } else {
        next.delete(taskId);
      }
      return next;
    });
  };

  const handleTaskPageSizeChange = (nextPageSize) => {
    setTaskPage(1);
    setTaskPageSize(nextPageSize);
    setTaskHasMore(false);
    setTaskNextCursor('');
    taskCursorHistoryRef.current = [''];
  };

  const handleVideoTaskPageSizeChange = (nextPageSize) => {
    setVideoTaskPage(1);
    setVideoTaskPageSize(nextPageSize);
  };

  // 全选/取消全选
  const handleSelectAll = (checked) => {
    if (checked) {
      setSelectedTaskIds(new Set(tasks.map((t) => t.id)));
    } else {
      setSelectedTaskIds(new Set());
    }
  };

  const handleSelectAllVideo = (checked) => {
    if (checked) {
      setVideoSelectedTaskIds(new Set(videoTasks.map((task) => task.id)));
    } else {
      setVideoSelectedTaskIds(new Set());
    }
  };

  // 批量删除任务
  const handleBatchDelete = async () => {
    const selectedTasks = tasks.filter((task) => selectedTaskIds.has(task.id));
    const undeletableTasks = selectedTasks.filter(
      (task) => !taskIsDeletable(task),
    );
    if (selectedTaskIds.size === 0) {
      showError(t('请先选择要删除的任务'));
      return;
    }
    if (undeletableTasks.length > 0) {
      showError(t('运行中的任务暂不支持删除，请等待完成后再删除'));
      return;
    }

    setDeletingTasks(true);
    try {
      const deletePromises = Array.from(selectedTaskIds).map((taskId) =>
        API.delete(`/api/image-generation/tasks/${taskId}`),
      );

      const results = await Promise.allSettled(deletePromises);
      const successCount = results.filter(
        (r) => r.status === 'fulfilled',
      ).length;
      const failCount = results.filter((r) => r.status === 'rejected').length;

      if (successCount > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successCount }));
        const deletedTaskIds = new Set(
          Array.from(selectedTaskIds).filter(
            (taskId, index) => results[index]?.status === 'fulfilled',
          ),
        );
        setTasks((prevTasks) =>
          prevTasks.filter((task) => !deletedTaskIds.has(task.id)),
        );
        setTaskTotal((prev) => Math.max(0, prev - successCount));
        if (selectedTask && deletedTaskIds.has(selectedTask.id)) {
          taskDetailRequestSeqRef.current += 1;
          setTaskModalVisible(false);
          setSelectedTask(null);
        }
        setSelectedTaskIds(new Set());
      }

      if (failCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failCount }));
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setDeletingTasks(false);
    }
  };

  const handleVideoBatchDelete = async () => {
    if (videoSelectedTaskIds.size === 0) {
      showError(t('请先选择要删除的任务'));
      return;
    }
    const selectedVideoTasks = videoTasks.filter((task) =>
      videoSelectedTaskIds.has(task.id),
    );
    if (
      selectedVideoTasks.some(
        (task) => task.status === 'queued' || task.status === 'in_progress',
      )
    ) {
      showError(t('运行中的任务暂不支持删除，请等待完成后再删除'));
      return;
    }
    setDeletingVideoTasks(true);
    try {
      const results = await Promise.allSettled(
        Array.from(videoSelectedTaskIds).map((taskId) =>
          API.delete(`/api/video-generation/tasks/${taskId}`),
        ),
      );
      const successIds = new Set(
        Array.from(videoSelectedTaskIds).filter(
          (taskId, index) => results[index]?.status === 'fulfilled',
        ),
      );
      if (successIds.size > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successIds.size }));
        setVideoTasks((prev) =>
          prev.filter((task) => !successIds.has(task.id)),
        );
        setVideoTaskTotal((prev) => Math.max(0, prev - successIds.size));
        setVideoSelectedTaskIds(new Set());
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setDeletingVideoTasks(false);
    }
  };

  // 连接 SSE
  const connectSSE = () => {
    try {
      const eventSource = new EventSource('/api/image-generation/sse', {
        withCredentials: true,
      });

      eventSource.onopen = () => {
        setSseConnected(true);
      };

      eventSource.addEventListener('task_update', (e) => {
        try {
          const data = JSON.parse(e.data);
          updateTaskInList(data);
        } catch (err) {
          console.error('Failed to parse SSE data:', err);
        }
      });

      eventSource.onerror = () => {
        console.log('SSE connection error, falling back to polling');
        eventSource.close();
        sseRef.current = null;
        setSseConnected(false);
        startPolling();
      };

      sseRef.current = eventSource;
    } catch (error) {
      console.error('Failed to connect SSE:', error);
      startPolling();
    }
  };

  // 断开 SSE
  const disconnectSSE = () => {
    if (sseRef.current) {
      sseRef.current.close();
      sseRef.current = null;
    }
    setSseConnected(false);
  };

  // 开始轮询
  const startPolling = () => {
    if (pollingTimerRef.current) return;

    pollingTimerRef.current = setInterval(() => {
      if (generationMode === CANVAS_MODE_VIDEO) {
        loadVideoTasks();
        return;
      }
      if (generationMode !== CANVAS_MODE_IMAGE) {
        return;
      }
      if (isDefaultTaskViewState(taskListStateRef.current)) {
        loadTaskUpdates();
        return;
      }
      loadTasks(true); // 静默刷新，不触发 Spin 遮罩
    }, pollingIntervalRef.current * 1000);
  };

  // 停止轮询
  const stopPolling = () => {
    if (pollingTimerRef.current) {
      clearInterval(pollingTimerRef.current);
      pollingTimerRef.current = null;
    }
  };

  useEffect(() => {
    const shouldPoll =
      isPageVisible &&
      (generationMode === CANVAS_MODE_VIDEO
        ? hasActiveVideoTasks
        : generationMode === CANVAS_MODE_IMAGE
          ? !sseConnected && hasActiveTasks
          : false);
    if (!shouldPoll) {
      stopPolling();
      return undefined;
    }

    startPolling();
    return () => stopPolling();
  }, [
    generationMode,
    hasActiveTasks,
    hasActiveVideoTasks,
    isPageVisible,
    pollingIntervalSeconds,
    sseConnected,
  ]);

  // 更新任务列表中的单个任务
  const updateTaskInList = (updatedTask) => {
    setTasks((prevTasks) => {
      const index = prevTasks.findIndex((t) => t.id === updatedTask.id);
      if (index !== -1) {
        const newTasks = [...prevTasks];
        newTasks[index] = {
          ...newTasks[index],
          ...updatedTask,
        };
        return newTasks;
      }
      if (!isDefaultTaskViewState(taskListStateRef.current)) {
        return prevTasks;
      }
      return mergeTaskCollections(prevTasks, [updatedTask], taskPageSize);
    });
    setSelectedTask((prevTask) => {
      if (!prevTask || prevTask.id !== updatedTask.id) {
        return prevTask;
      }
      return {
        ...prevTask,
        ...updatedTask,
      };
    });
  };

  const updateVideoTaskInList = (updatedTask) => {
    setVideoTasks((prev) => {
      const index = prev.findIndex((task) => task.id === updatedTask.id);
      if (index === -1) {
        if (
          videoTaskPage === 1 &&
          !videoTaskStatusFilter &&
          !videoTaskModelFilter &&
          !videoTaskTimeFilter
        ) {
          return [updatedTask, ...prev].slice(0, videoTaskPageSize);
        }
        return prev;
      }
      const next = [...prev];
      next[index] = {
        ...next[index],
        ...updatedTask,
      };
      return next;
    });
    if (
      videoTaskPage === 1 &&
      !videoTaskStatusFilter &&
      !videoTaskModelFilter &&
      !videoTaskTimeFilter &&
      !videoTasks.some((task) => task.id === updatedTask.id)
    ) {
      setVideoTaskTotal((prev) => prev + 1);
    }
    setVideoSelectedTask((prev) => {
      if (!prev || prev.id !== updatedTask.id) {
        return prev;
      }
      return {
        ...prev,
        ...updatedTask,
      };
    });
  };

  useEffect(() => {
    // 模型列表尚未加载时跳过，避免以空列表覆盖从 localStorage 恢复的选择
    if (models.length === 0) return;
    const isModelEnabled = (model) =>
      model.status === undefined || model.status === null || model.status === 1;
    let filtered = [];
    if (selectedSeries === 'all') {
      filtered = models.filter(isModelEnabled);
    } else if (selectedSeries) {
      filtered = models.filter(
        (model) =>
          model.model_series === selectedSeries && isModelEnabled(model),
      );
    }
    setFilteredModels(filtered);
    setSelectedModel((current) => {
      if (filtered.some((model) => model.request_model === current)) {
        return current;
      }
      return filtered[0]?.request_model || '';
    });
    if (filtered.length === 0) {
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
    }
  }, [selectedSeries, models]);

  useEffect(() => {
    const prefill = pendingCanvasPrefillRef.current;
    if (!prefill || groupLoading) {
      return;
    }

    if (prefill.selected_group && prefill.selected_group !== selectedGroup) {
      const canUsePrefillGroup = groupOptions.some(
        (group) =>
          group.group === prefill.selected_group &&
          group.has_available_token !== false,
      );
      if (canUsePrefillGroup) {
        setSelectedGroup(prefill.selected_group);
        return;
      }
      if (
        prefill.selected_group &&
        !prefillGroupFallbackNoticeShownRef.current
      ) {
        showSuccess(t('原作品分组当前不可用，已切换到默认可用分组'));
        prefillGroupFallbackNoticeShownRef.current = true;
      }
    }

    if (models.length === 0) {
      return;
    }
    if (loadedModelsGroupRef.current !== selectedGroup) {
      return;
    }

    const params = (() => {
      if (!prefill.params) {
        return {};
      }
      if (typeof prefill.params === 'object') {
        return prefill.params;
      }
      try {
        return JSON.parse(prefill.params);
      } catch (error) {
        return {};
      }
    })();

    if (prefill.prompt) {
      setInspiration(prefill.prompt);
    }
    if (params.aspect_ratio) {
      setAspectRatio(params.aspect_ratio);
    }
    if (params.resolution || params.image_size || params.imageSize) {
      setResolution(params.resolution || params.image_size || params.imageSize);
    }
    if (params.n || params.quantity) {
      setQuantity(normalizeTaskCount(params.n || params.quantity));
    }
    if (prefill.mode === 'reference' && prefill.image_url) {
      const remoteReference = buildRemoteReferenceFile(prefill.image_url);
      if (remoteReference) {
        setReferenceImages([remoteReference]);
      }
    }

    const syncSeriesForModel = (model) => {
      if (model?.model_series) {
        setSelectedSeries(model.model_series);
      }
    };

    const supportsGeneration = (model) =>
      !!model && modelSupportsCapability(model, IMAGE_CAPABILITY_GENERATION);
    const supportsEditing = (model) =>
      !!model && modelSupportsCapability(model, IMAGE_CAPABILITY_EDITING);
    const exactModel = models.find(
      (model) => model.request_model === prefill.model_id,
    );
    if (prefill.mode === 'reference') {
      if (exactModel && supportsEditing(exactModel)) {
        setSelectedModel(exactModel.request_model);
        syncSeriesForModel(exactModel);
      } else {
        const fallbackEditModel = models.find((model) =>
          supportsEditing(model),
        );
        if (fallbackEditModel) {
          setSelectedModel(fallbackEditModel.request_model);
          syncSeriesForModel(fallbackEditModel);
          showSuccess(
            t('原作品模型当前不可用于参考图继续创作，已切换到可编辑模型'),
          );
        } else {
          showError(t('当前分组下没有支持参考图编辑的模型，请切换分组或模型'));
        }
      }
    } else if (exactModel && supportsGeneration(exactModel)) {
      setSelectedModel(exactModel.request_model);
      syncSeriesForModel(exactModel);
    } else {
      const fallbackGenerationModel = models.find((model) =>
        supportsGeneration(model),
      );
      if (fallbackGenerationModel) {
        setSelectedModel(fallbackGenerationModel.request_model);
        syncSeriesForModel(fallbackGenerationModel);
        showSuccess(t('原作品模型当前不可用，已切换到当前分组的可用模型'));
      } else {
        showError(t('当前分组下没有支持生图的模型，请切换分组或模型'));
      }
    }

    pendingCanvasPrefillRef.current = null;
    prefillGroupFallbackNoticeShownRef.current = false;
    try {
      sessionStorage.removeItem(CANVAS_PREFILL_STORAGE_KEY);
    } catch (error) {
      console.error('Failed to clear canvas prefill state:', error);
    }
  }, [groupLoading, groupOptions, models, selectedGroup, t]);

  useEffect(() => {
    if (videoModels.length === 0) return;
    const filtered =
      videoSelectedSeries === 'all'
        ? videoModels
        : videoModels.filter(
            (model) => model.model_series === videoSelectedSeries,
          );
    setVideoFilteredModels(filtered);
    setVideoSelectedModel((current) => {
      if (filtered.some((model) => model.request_model === current)) {
        return current;
      }
      return filtered[0]?.request_model || '';
    });
  }, [videoSelectedSeries, videoModels]);

  // 保存用户选择到 localStorage
  useEffect(() => {
    if (selectedGroup) {
      try {
        localStorage.setItem(STORAGE_KEYS.GROUP, selectedGroup);
      } catch (e) {
        console.error('Failed to save selectedGroup to localStorage:', e);
      }
    }
  }, [selectedGroup]);

  useEffect(() => {
    if (selectedSeries) {
      try {
        localStorage.setItem(STORAGE_KEYS.SERIES, selectedSeries);
      } catch (e) {
        console.error('Failed to save selectedSeries to localStorage:', e);
      }
    }
  }, [selectedSeries]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.MODE, generationMode);
    } catch (e) {
      console.error('Failed to save generation mode:', e);
    }
  }, [generationMode]);

  useEffect(() => {
    if (selectedModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.MODEL, selectedModel);
      } catch (e) {
        console.error('Failed to save selectedModel to localStorage:', e);
      }
    }
  }, [selectedModel]);

  useEffect(() => {
    if (videoSelectedSeries) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_SERIES, videoSelectedSeries);
      } catch (e) {
        console.error('Failed to save videoSelectedSeries to localStorage:', e);
      }
    }
  }, [videoSelectedSeries]);

  useEffect(() => {
    if (videoSelectedModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_MODEL, videoSelectedModel);
      } catch (e) {
        console.error('Failed to save videoSelectedModel to localStorage:', e);
      }
    }
  }, [videoSelectedModel]);

  useEffect(() => {
    if (chatModel) {
      try {
        localStorage.setItem(STORAGE_KEYS.CHAT_MODEL, chatModel);
      } catch (e) {
        console.error('Failed to save chatModel:', e);
      }
    }
  }, [chatModel]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.CHAT_TEMPERATURE, chatTemperature);
    } catch (e) {
      console.error('Failed to save chatTemperature:', e);
    }
  }, [chatTemperature]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.CHAT_CONTEXT, chatContext);
    } catch (e) {
      console.error('Failed to save chatContext:', e);
    }
  }, [chatContext]);

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEYS.CHAT_TOOLS,
        chatToolsEnabled ? 'true' : 'false',
      );
    } catch (e) {
      console.error('Failed to save chatToolsEnabled:', e);
    }
  }, [chatToolsEnabled]);

  useEffect(() => {
    if (aspectRatio) {
      try {
        localStorage.setItem(STORAGE_KEYS.ASPECT_RATIO, aspectRatio);
      } catch (e) {
        console.error('Failed to save aspectRatio to localStorage:', e);
      }
    }
  }, [aspectRatio]);

  useEffect(() => {
    if (resolution) {
      try {
        localStorage.setItem(STORAGE_KEYS.RESOLUTION, resolution);
      } catch (e) {
        console.error('Failed to save resolution to localStorage:', e);
      }
    }
  }, [resolution]);

  useEffect(() => {
    if (videoAspectRatio) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_ASPECT_RATIO, videoAspectRatio);
      } catch (e) {
        console.error('Failed to save videoAspectRatio:', e);
      }
    }
  }, [videoAspectRatio]);

  useEffect(() => {
    if (videoResolution) {
      try {
        localStorage.setItem(STORAGE_KEYS.VIDEO_RESOLUTION, videoResolution);
      } catch (e) {
        console.error('Failed to save videoResolution:', e);
      }
    }
  }, [videoResolution]);

  useEffect(() => {
    try {
      localStorage.setItem(
        STORAGE_KEYS.VIDEO_DURATION,
        String(videoDuration || 0),
      );
    } catch (e) {
      console.error('Failed to save videoDuration:', e);
    }
  }, [videoDuration]);

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEYS.QUANTITY, quantity.toString());
    } catch (e) {
      console.error('Failed to save quantity to localStorage:', e);
    }
  }, [quantity]);

  useEffect(() => {
    if (selectedModel) {
      const model = models.find((m) => m.request_model === selectedModel);
      if (model) {
        setSelectedModelData(model);

        if (model.aspect_ratios) {
          try {
            const ratios = JSON.parse(model.aspect_ratios);
            setAvailableAspectRatios(ratios);
            // 只在当前 aspectRatio 为空或不在新���表中时才重置
            setAspectRatio((current) => {
              if (!current || !ratios.includes(current)) {
                return ratios.length > 0 ? ratios[0] : '';
              }
              return current;
            });
          } catch (e) {
            setAvailableAspectRatios([]);
            setAspectRatio('');
          }
        } else {
          setAvailableAspectRatios([]);
          setAspectRatio('');
        }

        if (model.resolutions) {
          try {
            const resolutions = JSON.parse(model.resolutions);
            setAvailableResolutions(resolutions);
            // 只在当前 resolution 为空或不在新列表中时才重置
            setResolution((current) => {
              if (!current || !resolutions.includes(current)) {
                return resolutions.length > 0 ? resolutions[0] : '';
              }
              return current;
            });
          } catch (e) {
            setAvailableResolutions([]);
            setResolution('');
          }
        } else {
          setAvailableResolutions([]);
          setResolution('');
        }
      }
    } else {
      setSelectedModelData(null);
      setAvailableAspectRatios([]);
      setAvailableResolutions([]);
      setAspectRatio('');
      setResolution('');
    }
  }, [selectedModel, models]);

  useEffect(() => {
    if (!videoSelectedModel) {
      setVideoSelectedModelData(null);
      setVideoAvailableAspectRatios([]);
      setVideoAvailableResolutions([]);
      return;
    }
    const model = videoModels.find(
      (item) => item.request_model === videoSelectedModel,
    );
    if (!model) {
      return;
    }
    setVideoSelectedModelData(model);
    setVideoAvailableAspectRatios(model.aspect_ratios || []);
    setVideoAvailableResolutions(model.resolutions || []);
    setVideoAspectRatio((current) => {
      if (!current || !(model.aspect_ratios || []).includes(current)) {
        return model.aspect_ratios?.[0] || '';
      }
      return current;
    });
    setVideoResolution((current) => {
      if (!current || !(model.resolutions || []).includes(current)) {
        return model.resolutions?.[0] || '';
      }
      return current;
    });
    setVideoDuration((current) => {
      if (!current || !(model.duration_options || []).includes(current)) {
        return model.duration_options?.[0] || 0;
      }
      return current;
    });
  }, [videoSelectedModel, videoModels]);

  useEffect(() => {
    if (
      !selectedModelData ||
      !modelSupportsImageEditing(selectedModelData)
    ) {
      setReferenceImages([]);
      setMaskImage(null);
    }
    if (!selectedModelData || !modelSupportsMaskEditing(selectedModelData)) {
      setMaskImage(null);
    }
  }, [selectedModelData]);

  useEffect(() => {
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit <= 0 || referenceImages.length <= limit) {
      return;
    }

    setReferenceImages((prev) => prev.slice(0, limit));
    showError(
      t('当前模型最多只能上传 {{count}} 张参考图', {
        count: limit,
      }),
    );
  }, [selectedModelData, referenceImages.length, t]);

  useEffect(() => {
    if (referenceImages.length === 0) {
      setMaskImage(null);
    }
  }, [referenceImages]);

  const handleImageUpload = ({ fileList }) => {
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit > 0 && fileList.length > limit) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: limit,
        }),
      );
      return;
    }
    setReferenceImages(fileList);
  };

  const handleMaskUpload = ({ fileList }) => {
    setMaskImage(fileList[0] || null);
  };

  const validateImageSize = (file) => {
    const fileSizeMB = file.size / 1024 / 1024;
    if (fileSizeMB > maxImageSize) {
      showError(
        t('图片文件过大：{{size}}MB，最大允许 {{max}}MB', {
          size: fileSizeMB.toFixed(2),
          max: maxImageSize,
        }),
      );
      return false;
    }
    return true;
  };

  const handleImageRemove = (file) => {
    setReferenceImages(referenceImages.filter((img) => img.uid !== file.uid));
  };

  const handleMaskRemove = () => {
    setMaskImage(null);
  };

  const handleVideoReferenceUpload = ({ fileList }) => {
    setVideoReferenceImage(fileList[0] || null);
  };

  const handleVideoReferenceRemove = () => {
    setVideoReferenceImage(null);
  };

  const normalizeTaskCount = (value) => {
    const count = Number(value);
    if (!Number.isFinite(count)) {
      return 1;
    }
    return Math.min(DEFAULT_MAX_BATCH_TASKS, Math.max(1, Math.floor(count)));
  };

  const handleGenerate = async () => {
    const supportsImageGeneration = modelSupportsCapability(
      selectedModelData,
      IMAGE_CAPABILITY_GENERATION,
    );
    const supportsImageEditing = modelSupportsCapability(
      selectedModelData,
      IMAGE_CAPABILITY_EDITING,
    );

    if (!selectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!selectedGroup) {
      showError(t('请选择分组'));
      return;
    }
    const selectedGroupOption = groupOptions.find(
      (item) => item.group === selectedGroup,
    );
    if (
      selectedGroupOption &&
      selectedGroupOption.has_available_token === false
    ) {
      showError(
        t('当前分组没有可用令牌，请前往 /console/token 创建或启用该分组令牌'),
      );
      return;
    }
    if (!inspiration.trim()) {
      showError(t('请输入灵感'));
      return;
    }
    if (!selectedModelData?.request_endpoint) {
      showError(t('模型配置错误：缺少 request_endpoint'));
      return;
    }
    if (showImageAspectRatioSelector && !aspectRatio) {
      showError(t('请选择宽高比'));
      return;
    }
    if (showImageResolutionSelector && !resolution) {
      showError(t('请选择分辨率'));
      return;
    }
    if (referenceImages.length > 0 && !supportsImageEditing) {
      showError(t('当前模型不支持图像编辑'));
      return;
    }
    const referenceImageLimit = getReferenceImageLimit(selectedModelData);
    if (
      referenceImageLimit > 0 &&
      referenceImages.length > referenceImageLimit
    ) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: referenceImageLimit,
        }),
      );
      return;
    }
    if (maskImage && referenceImages.length === 0) {
      showError(t('请先上传参考图再添加遮罩'));
      return;
    }
    if (!supportsImageGeneration && referenceImages.length === 0) {
      showError(t('当前模型至少需要上传一张参考图'));
      return;
    }

    setGenerating(true);
    try {
      const taskCount = normalizeTaskCount(quantity);
      if (taskCount > DEFAULT_MAX_BATCH_TASKS) {
        showError(
          t('单次最多可创建 {{count}} 个任务', {
            count: DEFAULT_MAX_BATCH_TASKS,
          }),
        );
        return;
      }

      // 准备参数对象
      const params = {};

      if (aspectRatio) {
        params.aspect_ratio = aspectRatio;
      }
      if (resolution) {
        params.resolution = resolution;
      }

      // 处理参考图片
      if (supportsImageEditing && referenceImages.length > 0) {
        const imagePromises = referenceImages.map((file) => {
          if (!file.fileInstance && file.url) {
            return Promise.resolve(file.url);
          }
          return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = (e) => resolve(e.target.result);
            reader.onerror = reject;
            reader.readAsDataURL(file.fileInstance);
          });
        });
        const base64Images = await Promise.all(imagePromises);
        params.reference_images = base64Images;
      }
      if (modelSupportsMaskEditing(selectedModelData) && maskImage?.fileInstance) {
        params.mask = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(maskImage.fileInstance);
        });
      }

      // UI uses inspiration wording; backend task DTO still expects prompt.
      const taskPayload = {
        model_id: selectedModel,
        group: selectedGroup,
        prompt: inspiration.trim(),
        request_endpoint: selectedModelData.request_endpoint,
        params: JSON.stringify(params),
      };

      const results = await Promise.allSettled(
        Array.from({ length: taskCount }, () =>
          API.post('/api/image-generation/tasks', taskPayload),
        ),
      );

      const createdTasks = [];
      let firstError = '';

      results.forEach((result) => {
        if (result.status === 'fulfilled' && result.value.data.success) {
          if (result.value.data.data) {
            createdTasks.push(result.value.data.data);
          }
          return;
        }

        if (!firstError) {
          firstError =
            result.status === 'fulfilled'
              ? result.value.data.message
              : result.reason?.response?.data?.message ||
                result.reason?.message;
        }
      });

      if (createdTasks.length > 0) {
        showSuccess(
          createdTasks.length === 1
            ? t('任务已创建，正在生成中...')
            : t('已创建 {{count}} 个任务，正在生成中...', {
                count: createdTasks.length,
              }),
        );
        const isDefaultTaskView =
          taskPage === 1 &&
          !taskStatusFilter &&
          !taskModelFilter &&
          !taskTimeFilter &&
          taskSortBy === 'created_time' &&
          taskSortOrder === 'desc';

        // 默认首页视图直接本地插入，避免一次整页回刷
        if (isDefaultTaskView) {
          setTasks((prevTasks) =>
            mergeTaskCollections(prevTasks, createdTasks, taskPageSize),
          );
          if (
            selectedTask &&
            createdTasks.some((task) => task.id === selectedTask.id)
          ) {
            setSelectedTask(
              createdTasks.find((task) => task.id === selectedTask.id) ||
                selectedTask,
            );
          }
        }
        setTaskTotal((prev) => prev + createdTasks.length);
        setSelectedTask(createdTasks[0]);
        loadTasks(false, {
          disableDuplicate: true,
          forceRefresh: true,
        });
      }

      if (createdTasks.length === 0) {
        showError(firstError || t('创建任务失败'));
      } else if (createdTasks.length < taskCount) {
        showError(
          t('部分任务创建失败：已创建 {{success}} / {{total}} 个任务', {
            success: createdTasks.length,
            total: taskCount,
          }),
        );
      }
    } catch (error) {
      const serverMessage =
        error.response?.data?.message || error.message || '';
      if (
        serverMessage.includes('valid user token') ||
        serverMessage.includes('no valid token') ||
        serverMessage.includes('current group has no valid token')
      ) {
        showError(
          serverMessage.includes('current group has no valid token')
            ? t(
                '当前分组没有可用令牌，请前往 /console/token 创建或启用该分组令牌',
              )
            : userCustomWorkerKeyEnabled
              ? t('请先创建可用令牌，或检查你的自定义 Worker Key 是否可用')
              : t('请先创建一个可用令牌后再使用 /canvas'),
        );
      } else if (error.response?.data?.message) {
        showError(error.response.data.message);
      } else {
        showError(error.message || t('创建任务失败'));
      }
    } finally {
      setGenerating(false);
    }
  };

  const handleGenerateVideo = async () => {
    const supportsImageToVideo = modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
    );
    const supportsTextToVideo = modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_TEXT_TO_VIDEO,
    );

    if (!videoSelectedModel) {
      showError(t('请选择模型'));
      return;
    }
    if (!videoPrompt.trim()) {
      showError(t('请输入灵感'));
      return;
    }
    if (!videoSelectedModelData?.request_endpoint) {
      showError(t('模型配置错误：缺少 request_endpoint'));
      return;
    }
    if (!videoSelectedModelData?.duration_options?.length || !videoDuration) {
      showError(t('当前模型未配置可用时长选项'));
      return;
    }
    if (videoReferenceImage && !supportsImageToVideo) {
      showError(t('当前模型不支持图生视频'));
      return;
    }
    if (!videoReferenceImage && !supportsTextToVideo) {
      showError(t('当前模型需要上传一张参考图'));
      return;
    }

    setVideoGenerating(true);
    try {
      let base64Image = '';
      if (videoReferenceImage?.fileInstance) {
        base64Image = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(videoReferenceImage.fileInstance);
        });
      } else if (videoReferenceImage?.url) {
        base64Image = videoReferenceImage.url;
      }
      const params = {
        duration: videoDuration,
        resolution: videoResolution,
        aspect_ratio: videoAspectRatio,
      };
      if (base64Image) {
        params.reference_images = [base64Image];
      }
      const taskPayload = {
        model_id: videoSelectedModel,
        prompt: videoPrompt.trim(),
        request_endpoint: videoSelectedModelData.request_endpoint,
        params: JSON.stringify(params),
      };
      const res = await API.post('/api/video-generation/tasks', taskPayload);
      if (!res.data.success) {
        showError(res.data.message || t('创建视频任务失败'));
        return;
      }
      const newTask = res.data.data;
      showSuccess(t('视频任务已创建，正在生成中...'));
      setVideoSelectedTask(newTask);
      if (
        videoTaskPage === 1 &&
        !videoTaskStatusFilter &&
        !videoTaskModelFilter &&
        !videoTaskTimeFilter
      ) {
        setVideoTasks((prev) => [newTask, ...prev].slice(0, videoTaskPageSize));
      } else {
        loadVideoTasks();
      }
      setVideoTaskTotal((prev) => prev + 1);
    } catch (error) {
      showError(
        error.response?.data?.message || error.message || t('创建视频任务失败'),
      );
    } finally {
      setVideoGenerating(false);
    }
  };

  const formatTimestamp = (timestamp) => {
    if (!timestamp) {
      return '-';
    }
    const date = new Date(timestamp * 1000);
    const pad = (value) => String(value).padStart(2, '0');
    return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(
      date.getDate(),
    )} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
  };

  const summarizeText = (value, fallback = t('暂无提示词')) => {
    const text = String(value || '').trim();
    if (!text) {
      return fallback;
    }
    return text.length > 54 ? `${text.slice(0, 54)}...` : text;
  };

  const getTaskTitle = (task, fallback) =>
    summarizeText(task?.prompt || task?.title, fallback);

  const getAssetKey = (asset) => asset?.task_id || asset?.id;

  const getAssetImageUrl = (asset) => asset?.image_url || asset?.thumbnail_url;

  const handleModeChange = (mode) => {
    if (!CANVAS_MODES.includes(mode)) {
      return;
    }
    setGenerationMode(mode);
    setMobileTaskbarVisible(false);
  };

  const handleCreateChat = () => {
    const now = Math.floor(Date.now() / 1000);
    const nextId = `local-chat-${now}-${Math.random().toString(16).slice(2)}`;
    const nextTask = {
      id: nextId,
      title: t('新对话'),
      summary: t('从底部输入器发送第一条消息'),
      status: t('草稿'),
      updated_at: now,
      messages: [],
    };
    setChatTasks((prev) => [nextTask, ...prev]);
    setSelectedChatId(nextId);
    setGenerationMode(CANVAS_MODE_CHAT);
    setMobileTaskbarVisible(false);
  };

  const handleSendChatMessage = () => {
    const prompt = chatPrompt.trim();
    if (!prompt && chatAttachments.length === 0) {
      showError(t('请输入消息'));
      return;
    }
    const now = Math.floor(Date.now() / 1000);
    const attachments = chatAttachments.map((asset) => ({ ...asset }));
    const userMessage = {
      id: `msg-${now}-${Math.random().toString(16).slice(2)}`,
      role: 'user',
      content: prompt,
      created_at: now,
      attachments,
    };
    const assistantMessage = {
      id: `msg-${now}-placeholder`,
      role: 'assistant',
      content: t('已收到。'),
      created_at: now,
      status: 'placeholder',
    };
    setChatTasks((prev) =>
      prev.map((chat) => {
        if (chat.id !== selectedChatId) {
          return chat;
        }
        const nextMessages = [...(chat.messages || []), userMessage, assistantMessage];
        return {
          ...chat,
          title:
            chat.title === t('新对话') && prompt
              ? summarizeText(prompt, t('新对话'))
              : chat.title,
          summary: prompt || t('已添加素材引用'),
          status: t('本地草稿'),
          updated_at: now,
          messages: nextMessages,
        };
      }),
    );
    setChatPrompt('');
    setChatAttachments([]);
  };

  const handleNewImageTask = () => {
    setGenerationMode(CANVAS_MODE_IMAGE);
    setSelectedTask(null);
    setMobileTaskbarVisible(false);
  };

  const handleNewVideoTask = () => {
    setGenerationMode(CANVAS_MODE_VIDEO);
    setVideoSelectedTask(null);
    setMobileTaskbarVisible(false);
  };

  const downloadAsset = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      return;
    }
    const link = document.createElement('a');
    link.href = imageUrl;
    link.download = `image-${getAssetKey(asset) || Date.now()}.png`;
    link.target = '_blank';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const deleteAsset = async (asset) => {
    const assetKey = getAssetKey(asset);
    if (!assetKey) {
      return;
    }
    try {
      const res = await API.delete(`/api/image-generation/tasks/${assetKey}`);
      if (!res.data.success) {
        showError(res.data.message || t('删除失败'));
        return;
      }
      showSuccess(t('删除成功'));
      setCanvasAssets((prev) =>
        prev.filter((item) => getAssetKey(item) !== assetKey),
      );
      setCanvasAssetsTotal((prev) => Math.max(0, prev - 1));
      if (selectedAssetPreview && getAssetKey(selectedAssetPreview) === assetKey) {
        setSelectedAssetPreview(null);
      }
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  };

  const addAssetToChat = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      showError(t('该资产没有可用图片'));
      return;
    }
    const remoteReference = buildRemoteReferenceFile(imageUrl);
    if (!remoteReference) {
      return;
    }
    setChatAttachments((prev) => [
      ...prev,
      {
        ...remoteReference,
        uid: `chat-asset-${getAssetKey(asset) || Date.now()}`,
        name: asset.prompt || t('图片资产'),
      },
    ]);
    showSuccess(t('已插入当前对话'));
  };

  const addAssetToImageReferences = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      showError(t('该资产没有可用图片'));
      return;
    }
    if (!selectedModelSupportsEditing) {
      showError(t('当前模型不支持图像编辑'));
      return;
    }
    const limit = getReferenceImageLimit(selectedModelData);
    if (limit > 0 && referenceImages.length >= limit) {
      showError(
        t('当前模型最多只能上传 {{count}} 张参考图', {
          count: limit,
        }),
      );
      return;
    }
    const remoteReference = buildRemoteReferenceFile(imageUrl);
    if (!remoteReference) {
      return;
    }
    setReferenceImages((prev) => [
      ...prev,
      {
        ...remoteReference,
        uid: `asset-${getAssetKey(asset) || Date.now()}`,
        name: asset.prompt || t('图片资产'),
      },
    ]);
    showSuccess(t('已作为参考图插入图片任务'));
  };

  const addAssetToVideoReference = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    if (!imageUrl) {
      showError(t('该资产没有可用图片'));
      return;
    }
    if (!videoSelectedModelSupportsImageToVideo) {
      showError(t('当前模型不支持图生视频'));
      return;
    }
    const remoteReference = buildRemoteReferenceFile(imageUrl);
    if (!remoteReference) {
      return;
    }
    setVideoReferenceImage({
      ...remoteReference,
      uid: `video-asset-${getAssetKey(asset) || Date.now()}`,
      name: asset.prompt || t('首帧图'),
    });
    showSuccess(t('已作为首帧图插入视频任务'));
  };

  const insertAssetIntoCurrentMode = (asset) => {
    if (generationMode === CANVAS_MODE_CHAT) {
      addAssetToChat(asset);
      return;
    }
    if (generationMode === CANVAS_MODE_VIDEO) {
      addAssetToVideoReference(asset);
      return;
    }
    addAssetToImageReferences(asset);
  };

  const styles = {
    container: {
      display: 'flex',
      height: 'calc(100vh - 60px)',
      marginTop: 60,
      overflow: 'hidden',
      background: 'var(--semi-color-bg-1)',
    },
    leftPanel: {
      width: isMobile ? '100%' : 300,
      minWidth: isMobile ? 0 : 300,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
    },
    leftContent: {
      flex: 1,
      overflowY: 'auto',
      padding: 16,
    },
    rightPanel: {
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--semi-color-bg-1)',
      overflow: 'hidden',
    },
    rightContent: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      alignItems: 'stretch',
      justifyContent: 'stretch',
    },
    contentColumn: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    },
    stage: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'flex-end',
      padding: isMobile ? '20px 12px 28px' : '56px 24px 76px',
      overflowY: 'auto',
    },
    stageInner: {
      width: '100%',
      maxWidth: 980,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'stretch',
      gap: isMobile ? 12 : 16,
    },
    stageTitle: {
      color: '#f5f7fa',
      textAlign: 'center',
      fontSize: isMobile ? 18 : 24,
      fontWeight: 700,
      letterSpacing: 0,
      lineHeight: 1.2,
    },
    stageSubtitle: {
      color: 'rgba(245, 247, 250, 0.6)',
      textAlign: 'center',
      fontSize: isMobile ? 12 : 13,
      lineHeight: 1.5,
      maxWidth: 720,
      margin: '0 auto',
    },
    stagePanelWrap: {
      width: '100%',
      display: 'flex',
      justifyContent: 'center',
    },
    stagePanel: {
      width: '100%',
      maxWidth: 900,
      borderRadius: 12,
      border: '1px solid rgba(148, 163, 184, 0.2)',
      background: 'rgba(8, 10, 15, 0.88)',
      boxShadow: '0 28px 80px rgba(0, 0, 0, 0.38)',
      backdropFilter: 'blur(18px)',
      padding: isMobile ? '14px' : '18px',
    },
    promptArea: {
      borderRadius: 10,
      border: '1px solid rgba(148, 163, 184, 0.18)',
      background: 'rgba(13, 18, 28, 0.95)',
      padding: isMobile ? '10px 10px 12px' : '12px 12px 14px',
    },
    promptInput: {
      border: 'none',
      background: 'transparent',
      resize: 'none',
      boxShadow: 'none',
      color: '#f5f7fa',
      caretColor: '#fff',
      fontSize: 15,
      lineHeight: 1.55,
    },
    promptInputHint: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 8,
      alignItems: 'center',
      marginTop: 8,
      color: 'rgba(245, 247, 250, 0.48)',
      fontSize: 12,
      lineHeight: 1.4,
    },
    promptAssetBar: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minHeight: 36,
      marginBottom: 8,
    },
    uploadIconBtn: {
      width: 32,
      height: 32,
      minWidth: 32,
      borderRadius: 8,
      border: '1px dashed rgba(148, 163, 184, 0.36)',
      background: 'rgba(15, 23, 42, 0.72)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: '#dbeafe',
      transition: 'opacity 0.2s, border-color 0.2s, background 0.2s',
    },
    footer: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      marginTop: 12,
    },
    footerRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      flexWrap: 'wrap',
    },
    footerRowLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minWidth: 0,
    },
    footerRowRight: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      justifyContent: 'flex-end',
      minWidth: 0,
    },
    pillButton: {
      minHeight: 34,
      borderRadius: 999,
      border: '1px solid rgba(148, 163, 184, 0.24)',
      background: 'rgba(17, 24, 39, 0.88)',
      color: '#e5e7eb',
      padding: '0 12px',
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      fontSize: 13,
      lineHeight: 1,
    },
    pillButtonActive: {
      borderColor: 'rgba(99, 102, 241, 0.9)',
      background: 'rgba(30, 41, 59, 0.96)',
      color: '#fff',
    },
    pillButtonMuted: {
      color: 'rgba(226, 232, 240, 0.82)',
    },
    pillButtonLabel: {
      maxWidth: 170,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    darkMenu: {
      minWidth: 240,
      maxWidth: 340,
      border: '1px solid rgba(148, 163, 184, 0.2)',
      borderRadius: 12,
      background: '#0f172a',
      boxShadow: '0 24px 60px rgba(0, 0, 0, 0.45)',
      padding: 6,
    },
    darkMenuItem: {
      color: '#e5e7eb',
      borderRadius: 8,
      margin: 0,
      padding: '8px 10px',
      whiteSpace: 'nowrap',
    },
    darkMenuItemActive: {
      background: 'rgba(59, 130, 246, 0.18)',
      color: '#fff',
    },
    modelMenuItem: {
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      minWidth: 0,
    },
    modelMenuTitle: {
      fontSize: 13,
      lineHeight: 1.35,
      color: 'inherit',
      whiteSpace: 'normal',
      wordBreak: 'break-word',
    },
    modelMenuMeta: {
      fontSize: 11,
      lineHeight: 1.4,
      color: 'rgba(226, 232, 240, 0.66)',
      wordBreak: 'break-all',
    },
    compactField: {
      minWidth: 112,
    },
    label: {
      display: 'block',
      fontSize: 13,
      fontWeight: 500,
      color: 'var(--semi-color-text-0)',
      marginBottom: 6,
    },
    fieldGroup: {
      marginBottom: 16,
    },
    catalogHeader: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
      marginBottom: 14,
    },
    catalogTitle: {
      fontSize: 16,
      fontWeight: 650,
      color: 'var(--semi-color-text-0)',
    },
    catalogTools: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      marginBottom: 14,
    },
    modelSection: {
      marginBottom: 14,
    },
    modelSectionHeader: {
      width: '100%',
      minHeight: 36,
      border: 'none',
      background: 'transparent',
      padding: '6px 0',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      cursor: 'pointer',
      color: 'var(--semi-color-text-0)',
    },
    modelList: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    modelCard: {
      width: '100%',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      padding: '10px 12px',
      textAlign: 'left',
      cursor: 'pointer',
      transition: 'border-color 0.2s, background 0.2s',
    },
    modelCardActive: {
      borderColor: 'var(--semi-color-primary)',
      background: 'var(--semi-color-primary-light-default)',
    },
    modelCardTitle: {
      display: 'block',
      marginBottom: 5,
      color: 'var(--semi-color-text-0)',
      fontSize: 15,
      fontWeight: 650,
      lineHeight: 1.35,
    },
    modelCardMeta: {
      display: 'block',
      color: 'var(--semi-color-text-2)',
      fontSize: 12,
      lineHeight: 1.4,
      wordBreak: 'break-all',
    },
    generateIconBtn: {
      width: 36,
      height: 36,
      minWidth: 36,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition: 'opacity 0.2s, background 0.2s, border-color 0.2s, color 0.2s',
    },
    addImageBtn: {
      width: 36,
      height: 36,
      borderRadius: 8,
      border: '1px dashed var(--semi-color-border)',
      background: 'rgba(15, 23, 42, 0.9)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: '#cbd5e1',
      transition: 'border-color 0.2s',
    },
    emptyState: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 12,
      width: '100%',
      height: '100%',
    },
    emptyIcon: {
      width: 64,
      height: 64,
      borderRadius: '50%',
      background: 'rgba(232, 89, 60, 0.12)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    paramItem: {
      width: isMobile ? 88 : 104,
      flexShrink: 0,
    },
    referenceImageThumb: {
      width: 36,
      height: 36,
      borderRadius: 8,
      objectFit: 'cover',
      border: '1px solid var(--semi-color-border)',
    },
    referenceImageContainer: {
      position: 'relative',
      display: 'inline-block',
    },
    removeImageBtn: {
      position: 'absolute',
      top: -6,
      right: -6,
      width: 18,
      height: 18,
      borderRadius: '50%',
      background: 'var(--semi-color-danger)',
      border: 'none',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: '#fff',
      fontSize: 10,
      padding: 0,
    },
    mobileOnlyFilters: {
      display: isMobile ? 'flex' : 'none',
      alignItems: 'center',
      gap: 8,
    },
    filterGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
    },
    filterLabel: {
      fontSize: 13,
      color: 'var(--semi-color-text-2)',
    },
    tasksGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
      gap: 20,
      padding: isMobile ? 12 : 20,
      width: '100%',
      alignContent: 'start',
      flexShrink: 0,
    },
    taskbarHeader: {
      padding: 14,
      borderBottom: '1px solid var(--semi-color-border)',
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
    },
    taskbarTitleRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
    },
    taskbarTitle: {
      fontSize: 15,
      fontWeight: 650,
      color: 'var(--semi-color-text-0)',
    },
    taskbarMeta: {
      color: 'var(--semi-color-text-2)',
      fontSize: 12,
    },
    taskList: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: 10,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    taskListItem: {
      width: '100%',
      border: '1px solid var(--semi-color-border)',
      borderRadius: 8,
      background: 'var(--semi-color-bg-0)',
      padding: 10,
      cursor: 'pointer',
      textAlign: 'left',
      display: 'flex',
      gap: 10,
      alignItems: 'center',
      transition: 'border-color 0.16s, background 0.16s',
    },
    taskListItemActive: {
      borderColor: 'var(--semi-color-primary)',
      background: 'var(--semi-color-primary-light-default)',
    },
    taskListText: {
      flex: 1,
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      gap: 5,
    },
    taskListTitle: {
      fontSize: 13,
      fontWeight: 600,
      lineHeight: 1.35,
      color: 'var(--semi-color-text-0)',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      display: '-webkit-box',
      WebkitLineClamp: 2,
      WebkitBoxOrient: 'vertical',
    },
    taskListMetaRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 6,
      minWidth: 0,
      color: 'var(--semi-color-text-2)',
      fontSize: 12,
    },
    taskThumb: {
      width: 48,
      height: 48,
      borderRadius: 8,
      objectFit: 'cover',
      flexShrink: 0,
      background: 'var(--semi-color-fill-0)',
      border: '1px solid var(--semi-color-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--semi-color-text-2)',
    },
    sidebarFooter: {
      padding: '10px 14px',
      borderTop: '1px solid var(--semi-color-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
    },
    workspaceTopbar: {
      height: 56,
      flexShrink: 0,
      borderBottom: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      padding: isMobile ? '0 10px' : '0 16px',
    },
    workspaceTopbarLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
    },
    modeSwitch: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
      padding: 4,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
    },
    modeButton: {
      minHeight: 32,
      border: 'none',
      borderRadius: 6,
      padding: isMobile ? '0 9px' : '0 12px',
      background: 'transparent',
      color: 'var(--semi-color-text-1)',
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      cursor: 'pointer',
      fontSize: 13,
      whiteSpace: 'nowrap',
    },
    modeButtonActive: {
      color: 'var(--semi-color-primary)',
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 1px 3px rgba(0, 0, 0, 0.08)',
    },
    workspaceBody: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    },
    mainViewport: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: isMobile ? 12 : 18,
      paddingBottom: isMobile ? 12 : 18,
    },
    composerDock: {
      flexShrink: 0,
      borderTop: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      padding: isMobile ? 10 : 14,
      boxShadow: '0 -8px 24px rgba(15, 23, 42, 0.06)',
    },
    composerShell: {
      width: '100%',
      maxWidth: 1080,
      margin: '0 auto',
      border: '1px solid var(--semi-color-border)',
      borderRadius: 8,
      background: '#10151f',
      padding: isMobile ? 10 : 12,
    },
    detailPanel: {
      border: '1px solid var(--semi-color-border)',
      borderRadius: 8,
      background: 'var(--semi-color-bg-0)',
      overflow: 'hidden',
    },
    detailHeader: {
      padding: isMobile ? 12 : 16,
      borderBottom: '1px solid var(--semi-color-border)',
      display: 'flex',
      alignItems: 'flex-start',
      justifyContent: 'space-between',
      gap: 12,
    },
    detailBody: {
      padding: isMobile ? 12 : 16,
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'minmax(0, 1.4fr) minmax(280px, 0.6fr)',
      gap: 16,
    },
    previewSurface: {
      minHeight: isMobile ? 240 : 420,
      borderRadius: 8,
      background: 'var(--semi-color-fill-0)',
      border: '1px solid var(--semi-color-border)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    previewImage: {
      width: '100%',
      height: '100%',
      maxHeight: isMobile ? 420 : 640,
      objectFit: 'contain',
      display: 'block',
    },
    detailMeta: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      minWidth: 0,
    },
    metaBlock: {
      display: 'flex',
      flexDirection: 'column',
      gap: 4,
      padding: 10,
      borderRadius: 8,
      background: 'var(--semi-color-fill-0)',
    },
    chatStream: {
      minHeight: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 12,
      maxWidth: 960,
      margin: '0 auto',
    },
    chatMessage: {
      maxWidth: '82%',
      borderRadius: 8,
      padding: '10px 12px',
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      color: 'var(--semi-color-text-0)',
      lineHeight: 1.55,
      wordBreak: 'break-word',
    },
    chatMessageUser: {
      alignSelf: 'flex-end',
      background: 'var(--semi-color-primary-light-default)',
      borderColor: 'var(--semi-color-primary-light-active)',
    },
    chatMessageAssistant: {
      alignSelf: 'flex-start',
    },
    assetGrid: {
      display: 'grid',
      gridTemplateColumns: isMobile
        ? 'repeat(2, minmax(0, 1fr))'
        : 'repeat(3, minmax(0, 1fr))',
      gap: 10,
    },
    assetCard: {
      border: '1px solid var(--semi-color-border)',
      borderRadius: 8,
      overflow: 'hidden',
      background: 'var(--semi-color-bg-0)',
      display: 'flex',
      flexDirection: 'column',
    },
    assetThumb: {
      width: '100%',
      aspectRatio: '1 / 1',
      objectFit: 'cover',
      background: 'var(--semi-color-fill-0)',
      display: 'block',
    },
    assetActions: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 6,
      padding: 8,
    },
    drawerFooterPager: {
      marginTop: 12,
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      gap: 8,
    },
  };

  const selectedGroupHasAvailableToken = !!groupOptions.find(
    (group) =>
      group.group === selectedGroup && group.has_available_token !== false,
  );
  const imageUiState = getCanvasImageUiState({
    model: selectedModelData,
    prompt: inspiration,
    referenceImages,
    selectedGroupHasAvailableToken,
    showImageAspectRatioSelector,
    showImageResolutionSelector,
    aspectRatio,
    resolution,
  });
  const selectedModelSupportsEditing = imageUiState.supportsEditing;
  const selectedModelSupportsMaskEditing = imageUiState.supportsMaskEditing;
  const selectedModelReferenceImageLimit =
    getReferenceImageLimit(selectedModelData);
  const hasReferenceImageLimit = selectedModelReferenceImageLimit > 0;
  const referenceImageLimitReached =
    hasReferenceImageLimit &&
    referenceImages.length >= selectedModelReferenceImageLimit;
  const videoSelectedModelSupportsImageToVideo =
    !!videoSelectedModelData &&
    modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_IMAGE_TO_VIDEO,
    );
  const videoSelectedModelSupportsTextToVideo =
    !!videoSelectedModelData &&
    modelSupportsVideoCapability(
      videoSelectedModelData,
      VIDEO_CAPABILITY_TEXT_TO_VIDEO,
    );
  const canGenerate =
    !!selectedModel &&
    !!selectedModelData &&
    imageUiState.canGenerate;

  const canGenerateVideo =
    !!videoSelectedModel &&
    !!videoSelectedModelData &&
    !!videoPrompt.trim() &&
    !!videoSelectedModelData?.duration_options?.length &&
    ((!!videoReferenceImage && videoSelectedModelSupportsImageToVideo) ||
      (!videoReferenceImage && videoSelectedModelSupportsTextToVideo));

  const selectedGroupOption = groupOptions.find(
    (group) => group.group === selectedGroup,
  );
  const selectedChat = chatTasks.find((chat) => chat.id === selectedChatId);
  const currentTaskSidebarTitle =
    generationMode === CANVAS_MODE_CHAT
      ? t('对话任务')
      : generationMode === CANVAS_MODE_VIDEO
        ? t('视频任务')
        : t('图片任务');

  const renderReferenceThumb = (file, onRemove) => (
    <div key={file.uid || file.name || file.url} style={styles.referenceImageContainer}>
      <img
        src={file.url || (file.fileInstance && URL.createObjectURL(file.fileInstance))}
        alt=''
        style={styles.referenceImageThumb}
      />
      <button
        type='button'
        aria-label={t('移除图片')}
        style={styles.removeImageBtn}
        onClick={onRemove}
      >
        <IconDelete size='extra-small' />
      </button>
    </div>
  );

  const renderPillDropdown = ({
    key,
    label,
    value,
    displayValue,
    options,
    onChange,
    disabled = false,
    emptyText = t('暂无可用选项'),
    extraContent = null,
  }) => {
    const menu = (
      <Dropdown.Menu style={styles.darkMenu}>
        {options.length > 0 ? (
          options.map((option) => {
            const selected = option.value === value;
            return (
              <Dropdown.Item
                key={option.value}
                style={{
                  ...styles.darkMenuItem,
                  ...(selected ? styles.darkMenuItemActive : null),
                }}
                onClick={() => onChange(option.value)}
              >
                {option.label}
              </Dropdown.Item>
            );
          })
        ) : (
          <div
            style={{
              ...styles.darkMenuItem,
              color: 'rgba(226, 232, 240, 0.66)',
            }}
          >
            {emptyText}
          </div>
        )}
        {extraContent ? (
          <div style={{ marginTop: 6, borderTop: '1px solid rgba(148, 163, 184, 0.16)' }}>
            {extraContent}
          </div>
        ) : null}
      </Dropdown.Menu>
    );

    return (
      <Dropdown key={key} trigger='click' position='bottomLeft' render={menu}>
        <button
          type='button'
          style={{
            ...styles.pillButton,
            ...(value !== '' && value !== null && value !== undefined
              ? styles.pillButtonActive
              : styles.pillButtonMuted),
            opacity: disabled ? 0.55 : 1,
            cursor: disabled ? 'not-allowed' : 'pointer',
          }}
          disabled={disabled}
        >
          <span style={styles.pillButtonLabel}>
            {label ? `${label} ${displayValue || t('请选择')}` : displayValue || t('请选择')}
          </span>
          <IconChevronDown size='small' />
        </button>
      </Dropdown>
    );
  };

  const renderModelDropdown = (isVideoMode, activeModelLabel) => {
    const modelOptions = isVideoMode ? enabledVideoModels : enabledImageModels;
    const selectedValue = isVideoMode ? videoSelectedModel : selectedModel;
    const handleSelect = (requestModel) => {
      const model = modelOptions.find((item) => item.request_model === requestModel);
      if (!model) {
        return;
      }
      if (isVideoMode) {
        selectVideoModelFromCatalog(model);
        return;
      }
      selectImageModelFromCatalog(model);
    };
    const menu = (
      <Dropdown.Menu style={{ ...styles.darkMenu, minWidth: isMobile ? 280 : 320 }}>
        {modelOptions.length > 0 ? (
          modelOptions.map((model) => {
            const selected = model.request_model === selectedValue;
            return (
              <Dropdown.Item
                key={model.request_model}
                style={{
                  ...styles.darkMenuItem,
                  ...(selected ? styles.darkMenuItemActive : null),
                }}
                onClick={() => handleSelect(model.request_model)}
              >
                <div style={styles.modelMenuItem}>
                  <span style={styles.modelMenuTitle}>
                    {getModelDisplayName(model)}
                  </span>
                  <span style={styles.modelMenuMeta}>
                    {model.request_model}
                  </span>
                </div>
              </Dropdown.Item>
            );
          })
        ) : (
          <div style={{ ...styles.darkMenuItem, color: 'rgba(226, 232, 240, 0.66)' }}>
            {isVideoMode ? t('当前没有可用视频模型') : t('当前分组下没有可用图片模型')}
          </div>
        )}
      </Dropdown.Menu>
    );

    return (
      <Dropdown trigger='click' position='bottomLeft' render={menu}>
        <button
          type='button'
          aria-label={isVideoMode ? t('选择视频模型') : t('选择图片模型')}
          data-canvas-model-selector={isVideoMode ? CANVAS_MODE_VIDEO : CANVAS_MODE_IMAGE}
          style={styles.pillButton}
          disabled={modelOptions.length === 0}
        >
          {isVideoMode ? <IconVideo size='small' /> : <IconImage size='small' />}
          <span style={{ ...styles.pillButtonLabel, maxWidth: isMobile ? 140 : 260 }}>
            {activeModelLabel}
          </span>
          <IconChevronDown size='small' />
        </button>
      </Dropdown>
    );
  };

  const renderStatusTag = (status, mode) => {
    const map =
      mode === CANVAS_MODE_VIDEO
        ? {
            completed: { color: 'green', text: t('已完成') },
            failed: { color: 'red', text: t('失败') },
            in_progress: { color: 'blue', text: t('生成中') },
            queued: { color: 'orange', text: t('等待中') },
          }
        : {
            success: { color: 'green', text: t('已完成') },
            failed: { color: 'red', text: t('失败') },
            generating: { color: 'blue', text: t('生成中') },
            pending: { color: 'orange', text: t('等待中') },
          };
    const meta = map[status] || { color: 'grey', text: status || t('未知') };
    return (
      <Tag color={meta.color} size='small'>
        {meta.text}
      </Tag>
    );
  };

  const renderSidebarEmpty = (title, description, action = null) => (
    <div style={{ padding: '28px 10px' }}>
      <Empty title={title} description={description} />
      {action ? (
        <div style={{ display: 'flex', justifyContent: 'center', marginTop: 12 }}>
          {action}
        </div>
      ) : null}
    </div>
  );

  const renderTaskThumb = (task, mode) => {
    const imageUrl =
      mode === CANVAS_MODE_VIDEO
        ? task.thumbnail_url
        : task.thumbnail_url || task.image_url;
    if (imageUrl) {
      return <img src={imageUrl} alt='' style={styles.taskThumb} />;
    }
    return (
      <div style={styles.taskThumb}>
        {mode === CANVAS_MODE_VIDEO ? <IconVideo /> : <IconImage />}
      </div>
    );
  };

  const renderChatTaskList = () => (
    <div style={styles.taskList}>
      {chatTasks.map((chat) => {
        const active = chat.id === selectedChatId;
        return (
          <div
            key={chat.id}
            role='button'
            tabIndex={0}
            style={{
              ...styles.taskListItem,
              ...(active ? styles.taskListItemActive : null),
            }}
            onClick={() => {
              setSelectedChatId(chat.id);
              setMobileTaskbarVisible(false);
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                setSelectedChatId(chat.id);
              }
            }}
          >
            <div style={styles.taskThumb}>
              <IconCommentStroked />
            </div>
            <div style={styles.taskListText}>
              <div style={styles.taskListTitle}>{chat.title}</div>
              <div style={styles.taskListMetaRow}>
                <span>{formatTimestamp(chat.updated_at)}</span>
                <Tag size='small'>{chat.status}</Tag>
              </div>
              <Text type='tertiary' size='small' ellipsis>
                {chat.summary}
              </Text>
            </div>
          </div>
        );
      })}
    </div>
  );

  const renderImageTaskList = () => (
    <>
      <div style={{ padding: '0 14px 10px' }}>
        <Select
          size='small'
          style={{ width: '100%' }}
          value={taskStatusFilter}
          onChange={setTaskStatusFilter}
        >
          <Select.Option value=''>{t('全部状态')}</Select.Option>
          <Select.Option value='pending'>{t('等待中')}</Select.Option>
          <Select.Option value='generating'>{t('生成中')}</Select.Option>
          <Select.Option value='success'>{t('已完成')}</Select.Option>
          <Select.Option value='failed'>{t('失败')}</Select.Option>
        </Select>
      </div>
      <Spin spinning={loadingTasks}>
        <div style={styles.taskList}>
          {taskListError
            ? renderSidebarEmpty(
                t('图片任务加载失败'),
                taskListError,
                <Button size='small' icon={<IconRefresh />} onClick={() => loadTasks()}>
                  {t('重试')}
                </Button>,
              )
            : tasks.length === 0
              ? renderSidebarEmpty(t('暂无图片任务'), t('使用底部输入器创建图片任务'))
              : tasks.map((task) => {
                  const active = selectedTask?.id === task.id;
                  return (
                    <div
                      key={task.id}
                      role='button'
                      tabIndex={0}
                      style={{
                        ...styles.taskListItem,
                        ...(active ? styles.taskListItemActive : null),
                      }}
                      onClick={() => {
                        handleTaskCardClick(task);
                        setMobileTaskbarVisible(false);
                      }}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          handleTaskCardClick(task);
                        }
                      }}
                    >
                      <div onClick={(event) => event.stopPropagation()}>
                        <Checkbox
                          checked={selectedTaskIds.has(task.id)}
                          onChange={(event) =>
                            handleTaskSelect(task.id, event.target.checked)
                          }
                        />
                      </div>
                      {renderTaskThumb(task, CANVAS_MODE_IMAGE)}
                      <div style={styles.taskListText}>
                        <div style={styles.taskListTitle}>
                          {getTaskTitle(task, t('图片任务'))}
                        </div>
                        <div style={styles.taskListMetaRow}>
                          <span>{formatTimestamp(task.created_time)}</span>
                          {renderStatusTag(task.status, CANVAS_MODE_IMAGE)}
                        </div>
                      </div>
                    </div>
                  );
                })}
        </div>
      </Spin>
    </>
  );

  const renderVideoTaskList = () => (
    <>
      <div style={{ padding: '0 14px 10px' }}>
        <Select
          size='small'
          style={{ width: '100%' }}
          value={videoTaskStatusFilter}
          onChange={setVideoTaskStatusFilter}
        >
          <Select.Option value=''>{t('全部状态')}</Select.Option>
          <Select.Option value='queued'>{t('等待中')}</Select.Option>
          <Select.Option value='in_progress'>{t('生成中')}</Select.Option>
          <Select.Option value='completed'>{t('已完成')}</Select.Option>
          <Select.Option value='failed'>{t('失败')}</Select.Option>
        </Select>
      </div>
      <Spin spinning={videoLoadingTasks}>
        <div style={styles.taskList}>
          {videoTaskListError
            ? renderSidebarEmpty(
                t('视频任务加载失败'),
                videoTaskListError,
                <Button
                  size='small'
                  icon={<IconRefresh />}
                  onClick={() => loadVideoTasks()}
                >
                  {t('重试')}
                </Button>,
              )
            : videoTasks.length === 0
              ? renderSidebarEmpty(t('暂无视频任务'), t('使用底部输入器创建视频任务'))
              : videoTasks.map((task) => {
                  const active = videoSelectedTask?.id === task.id;
                  return (
                    <div
                      key={task.id}
                      role='button'
                      tabIndex={0}
                      style={{
                        ...styles.taskListItem,
                        ...(active ? styles.taskListItemActive : null),
                      }}
                      onClick={() => {
                        handleVideoTaskCardClick(task);
                        setMobileTaskbarVisible(false);
                      }}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          handleVideoTaskCardClick(task);
                        }
                      }}
                    >
                      <div onClick={(event) => event.stopPropagation()}>
                        <Checkbox
                          checked={videoSelectedTaskIds.has(task.id)}
                          onChange={(event) =>
                            handleVideoTaskSelect(task.id, event.target.checked)
                          }
                        />
                      </div>
                      {renderTaskThumb(task, CANVAS_MODE_VIDEO)}
                      <div style={styles.taskListText}>
                        <div style={styles.taskListTitle}>
                          {getTaskTitle(task, t('视频任务'))}
                        </div>
                        <div style={styles.taskListMetaRow}>
                          <span>{formatTimestamp(task.created_time)}</span>
                          {renderStatusTag(task.status, CANVAS_MODE_VIDEO)}
                        </div>
                      </div>
                    </div>
                  );
                })}
        </div>
      </Spin>
    </>
  );

  const renderTaskPager = () => {
    if (generationMode === CANVAS_MODE_CHAT) {
      return (
        <Text type='tertiary' size='small'>
          {t('共 {{count}} 个对话', { count: chatTasks.length })}
        </Text>
      );
    }
    if (generationMode === CANVAS_MODE_VIDEO) {
      return (
        <>
          <Text type='tertiary' size='small'>
            {t('第 {{page}} 页 / 共 {{count}} 个', {
              page: videoTaskPage,
              count: videoTaskTotal,
            })}
          </Text>
          <div style={{ display: 'flex', gap: 6 }}>
            <Button
              size='small'
              disabled={videoTaskPage <= 1 || videoLoadingTasks}
              onClick={() => setVideoTaskPage((page) => Math.max(1, page - 1))}
            >
              {t('上一页')}
            </Button>
            <Button
              size='small'
              disabled={
                videoLoadingTasks ||
                videoTaskPage * videoTaskPageSize >= videoTaskTotal
              }
              onClick={() => setVideoTaskPage((page) => page + 1)}
            >
              {t('下一页')}
            </Button>
          </div>
        </>
      );
    }
    return (
      <>
        <Text type='tertiary' size='small'>
          {showsReliableTaskTotal
            ? t('第 {{page}} 页 / 共 {{count}} 个', {
                page: taskPage,
                count: taskTotal,
              })
            : t('第 {{page}} 页', { page: taskPage })}
        </Text>
        <div style={{ display: 'flex', gap: 6 }}>
          <Button
            size='small'
            disabled={taskPage <= 1 || loadingTasks}
            onClick={() => setTaskPage((page) => Math.max(1, page - 1))}
          >
            {t('上一页')}
          </Button>
          <Button
            size='small'
            disabled={loadingTasks || !hasNextTaskPage}
            onClick={() => setTaskPage((page) => page + 1)}
          >
            {t('下一页')}
          </Button>
        </div>
      </>
    );
  };

  const renderTaskSidebar = () => {
    const createConfig =
      generationMode === CANVAS_MODE_CHAT
        ? {
            text: t('新建对话'),
            icon: <IconCommentStroked />,
            action: handleCreateChat,
          }
        : generationMode === CANVAS_MODE_VIDEO
          ? {
              text: t('新建视频任务'),
              icon: <IconVideo />,
              action: handleNewVideoTask,
            }
          : {
              text: t('新建图片任务'),
              icon: <IconImage />,
              action: handleNewImageTask,
            };
    const selectedCount =
      generationMode === CANVAS_MODE_VIDEO
        ? videoSelectedTaskIds.size
        : selectedTaskIds.size;

    return (
      <div style={styles.leftPanel} data-canvas-task-sidebar={generationMode}>
        <div style={styles.taskbarHeader}>
          <div style={styles.taskbarTitleRow}>
            <div>
              <div style={styles.taskbarTitle}>{currentTaskSidebarTitle}</div>
              <div style={styles.taskbarMeta}>
                {generationMode === CANVAS_MODE_CHAT
                  ? t('当前模式的对话列表')
                  : generationMode === CANVAS_MODE_VIDEO
                    ? t('当前模式的视频生成记录')
                    : t('当前模式的图片生成记录')}
              </div>
            </div>
            <Button
              size='small'
              type='primary'
              icon={<IconPlus />}
              onClick={createConfig.action}
            >
              {createConfig.text}
            </Button>
          </div>
          {generationMode === CANVAS_MODE_IMAGE && selectedGroupOption ? (
            <Text
              type={
                selectedGroupOption.has_available_token === false
                  ? 'danger'
                  : 'tertiary'
              }
              size='small'
            >
              {selectedGroupOption.has_available_token === false
                ? t('当前图片分组暂无可用令牌')
                : t('图片分组：{{group}}', { group: selectedGroup })}
            </Text>
          ) : null}
        </div>

        {generationMode === CANVAS_MODE_CHAT
          ? renderChatTaskList()
          : generationMode === CANVAS_MODE_VIDEO
            ? renderVideoTaskList()
            : renderImageTaskList()}

        <div style={styles.sidebarFooter}>
          {generationMode !== CANVAS_MODE_CHAT && selectedCount > 0 ? (
            <Button
              size='small'
              type='danger'
              theme='outline'
              icon={<IconDelete />}
              loading={
                generationMode === CANVAS_MODE_VIDEO
                  ? deletingVideoTasks
                  : deletingTasks
              }
              onClick={
                generationMode === CANVAS_MODE_VIDEO
                  ? handleVideoBatchDelete
                  : handleBatchDelete
              }
            >
              {t('删除 {{count}} 项', { count: selectedCount })}
            </Button>
          ) : null}
          <div
            style={{
              marginLeft: 'auto',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}
          >
            {renderTaskPager()}
          </div>
        </div>
      </div>
    );
  };

  const renderMetaBlock = (label, value) => (
    <div style={styles.metaBlock}>
      <Text type='tertiary' size='small'>
        {label}
      </Text>
      <Text style={{ wordBreak: 'break-word' }}>{value || '-'}</Text>
    </div>
  );

  const renderReferenceStrip = (title, files) => {
    const visibleFiles = (files || []).filter(Boolean);
    if (visibleFiles.length === 0) {
      return null;
    }
    return (
      <div style={styles.metaBlock}>
        <Text type='tertiary' size='small'>
          {title}
        </Text>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          {visibleFiles.map((file) => (
            <img
              key={file.uid || file.name || file.url}
              src={file.url || (file.fileInstance && URL.createObjectURL(file.fileInstance))}
              alt=''
              style={styles.referenceImageThumb}
            />
          ))}
        </div>
      </div>
    );
  };

  const renderImagePreview = (task) => {
    if (!task) {
      return null;
    }
    if (task.status === 'success' && task.image_url) {
      return <img src={task.image_url} alt='' style={styles.previewImage} />;
    }
    if (task.status === 'failed') {
      return (
        <div style={styles.emptyState}>
          <IconAlertTriangle size='extra-large' />
          <Text type='danger'>{task.error_message || t('生成失败')}</Text>
        </div>
      );
    }
    return (
      <div style={styles.emptyState}>
        {task.status === 'generating' ? <Spin size='large' /> : <IconClock size='extra-large' />}
        <Text type='tertiary'>
          {task.status === 'generating' ? t('生成中') : t('等待中')}
        </Text>
        {task.status === 'generating' ? (
          <Progress
            percent={Number(task.progress) || 0}
            showInfo
            style={{ width: 220 }}
          />
        ) : null}
      </div>
    );
  };

  const renderImageDetail = () => {
    if (!selectedTask) {
      return (
        <div style={styles.detailPanel}>
          {renderSidebarEmpty(t('选择或创建图片任务'), t('左侧选择历史任务，或直接在底部输入器创建新图片'))}
        </div>
      );
    }
    return (
      <div style={styles.detailPanel}>
        <div style={styles.detailHeader}>
          <div style={{ minWidth: 0 }}>
            <Text strong style={{ display: 'block', fontSize: 16 }}>
              {getTaskTitle(selectedTask, t('图片任务'))}
            </Text>
            <Text type='tertiary' size='small'>
              {formatTimestamp(selectedTask.created_time)}
            </Text>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {renderStatusTag(selectedTask.status, CANVAS_MODE_IMAGE)}
            <Button size='small' onClick={() => setTaskModalVisible(true)}>
              {t('详情')}
            </Button>
            {selectedTask.image_url ? (
              <Button
                size='small'
                icon={<IconDownload />}
                onClick={() =>
                  downloadAsset({
                    id: selectedTask.id,
                    image_url: selectedTask.image_url,
                  })
                }
              >
                {t('下载')}
              </Button>
            ) : null}
          </div>
        </div>
        <div style={styles.detailBody}>
          <div style={styles.previewSurface}>{renderImagePreview(selectedTask)}</div>
          <div style={styles.detailMeta}>
            {renderMetaBlock(t('模型'), selectedTask.display_name || selectedTask.model_id)}
            {renderMetaBlock(t('提示词'), selectedTask.prompt)}
            {selectedTask.error_message
              ? renderMetaBlock(t('失败信息'), selectedTask.error_message)
              : null}
            {renderReferenceStrip(t('当前参考图'), referenceImages)}
            {renderReferenceStrip(t('当前遮罩'), maskImage ? [maskImage] : [])}
          </div>
        </div>
      </div>
    );
  };

  const renderVideoPreview = (task) => {
    if (!task) {
      return null;
    }
    const videoUrl = task.video_url || task.result_url;
    if (task.status === 'completed' && videoUrl) {
      return (
        <video
          src={videoUrl}
          poster={task.thumbnail_url}
          controls
          style={{ width: '100%', height: '100%', maxHeight: 640, background: '#000' }}
        />
      );
    }
    if (task.status === 'failed') {
      return (
        <div style={styles.emptyState}>
          <IconAlertTriangle size='extra-large' />
          <Text type='danger'>{task.fail_reason || t('生成失败')}</Text>
        </div>
      );
    }
    return (
      <div style={styles.emptyState}>
        {task.status === 'in_progress' ? <Spin size='large' /> : <IconPlayCircle size='extra-large' />}
        <Text type='tertiary'>
          {task.status === 'in_progress' ? t('生成中') : t('等待中')}
        </Text>
        <Progress
          percent={parseInt(String(task.progress || '0').replace('%', ''), 10) || 0}
          showInfo
          style={{ width: 220 }}
        />
      </div>
    );
  };

  const renderVideoDetail = () => {
    if (!videoSelectedTask) {
      return (
        <div style={styles.detailPanel}>
          {renderSidebarEmpty(t('选择或创建视频任务'), t('左侧选择历史任务，或直接在底部输入器创建新视频'))}
        </div>
      );
    }
    const videoUrl = videoSelectedTask.video_url || videoSelectedTask.result_url;
    return (
      <div style={styles.detailPanel}>
        <div style={styles.detailHeader}>
          <div style={{ minWidth: 0 }}>
            <Text strong style={{ display: 'block', fontSize: 16 }}>
              {getTaskTitle(videoSelectedTask, t('视频任务'))}
            </Text>
            <Text type='tertiary' size='small'>
              {formatTimestamp(videoSelectedTask.created_time)}
            </Text>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {renderStatusTag(videoSelectedTask.status, CANVAS_MODE_VIDEO)}
            <Button size='small' onClick={() => setVideoTaskModalVisible(true)}>
              {t('详情')}
            </Button>
            {videoUrl ? (
              <Button
                size='small'
                icon={<IconExternalOpen />}
                onClick={() => window.open(videoUrl, '_blank')}
              >
                {t('打开')}
              </Button>
            ) : null}
          </div>
        </div>
        <div style={styles.detailBody}>
          <div style={styles.previewSurface}>{renderVideoPreview(videoSelectedTask)}</div>
          <div style={styles.detailMeta}>
            {renderMetaBlock(
              t('模型'),
              videoSelectedTask.display_name || videoSelectedTask.model_id,
            )}
            {renderMetaBlock(t('提示词'), videoSelectedTask.prompt)}
            {renderMetaBlock(t('时长'), videoSelectedTask.duration ? `${videoSelectedTask.duration}s` : '-')}
            {videoSelectedTask.fail_reason
              ? renderMetaBlock(t('失败信息'), videoSelectedTask.fail_reason)
              : null}
            {renderReferenceStrip(
              t('当前首帧图 / 参考图'),
              videoReferenceImage ? [videoReferenceImage] : [],
            )}
          </div>
        </div>
      </div>
    );
  };

  const renderRecentTaskGrid = () => {
    if (generationMode === CANVAS_MODE_VIDEO) {
      if (videoTasks.length === 0) {
        return null;
      }
      return (
        <div style={{ ...styles.tasksGrid, gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', padding: 0 }}>
          {videoTasks.map((task) => (
            <VideoGenerationTaskCard
              key={task.id}
              task={task}
              selected={videoSelectedTaskIds.has(task.id)}
              onSelectChange={handleVideoTaskSelect}
              onClick={() => handleVideoTaskCardClick(task)}
            />
          ))}
        </div>
      );
    }
    if (tasks.length === 0) {
      return null;
    }
    return (
      <div style={{ ...styles.tasksGrid, gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', padding: 0 }}>
        {tasks.map((task) => (
          <ImageGenerationTaskCard
            key={task.id}
            task={task}
            selected={selectedTaskIds.has(task.id)}
            onSelectChange={handleTaskSelect}
            onClick={() => handleTaskCardClick(task)}
          />
        ))}
      </div>
    );
  };

  const renderChatWorkspace = () => {
    const messages = selectedChat?.messages || [];
    return (
      <div style={styles.chatStream}>
        {messages.length === 0 ? (
          <div style={styles.detailPanel}>
            {renderSidebarEmpty(t('对话工作区'), t('底部输入消息，可附加资产库图片作为上下文'))}
          </div>
        ) : (
          messages.map((message) => (
            <div
              key={message.id}
              style={{
                ...styles.chatMessage,
                ...(message.role === 'user'
                  ? styles.chatMessageUser
                  : styles.chatMessageAssistant),
              }}
            >
              <Text type='tertiary' size='small'>
                {message.role === 'user' ? t('你') : t('助手')}
              </Text>
              <div style={{ whiteSpace: 'pre-wrap', marginTop: 4 }}>
                {message.content || t('已添加素材引用')}
              </div>
              {message.attachments?.length ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 8 }}>
                  {message.attachments.map((asset) => (
                    <img
                      key={asset.uid || asset.url}
                      src={asset.url}
                      alt=''
                      style={styles.referenceImageThumb}
                    />
                  ))}
                </div>
              ) : null}
            </div>
          ))
        )}
      </div>
    );
  };

  const renderMainContent = () => (
    <div style={styles.mainViewport}>
      {generationMode === CANVAS_MODE_CHAT ? (
        renderChatWorkspace()
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {generationMode === CANVAS_MODE_VIDEO
            ? renderVideoDetail()
            : renderImageDetail()}
          {renderRecentTaskGrid()}
        </div>
      )}
    </div>
  );

  const renderModeSwitch = () => {
    const options = [
      {
        value: CANVAS_MODE_CHAT,
        label: t('对话'),
        icon: <IconCommentStroked size='small' />,
      },
      {
        value: CANVAS_MODE_IMAGE,
        label: t('图片'),
        icon: <IconImage size='small' />,
      },
      {
        value: CANVAS_MODE_VIDEO,
        label: t('视频'),
        icon: <IconVideo size='small' />,
      },
    ];
    return (
      <div style={styles.modeSwitch}>
        {options.map((option) => {
          const active = generationMode === option.value;
          return (
            <button
              key={option.value}
              type='button'
              aria-label={t('切换到{{mode}}模式', { mode: option.label })}
              aria-pressed={active}
              data-canvas-mode-button={option.value}
              style={{
                ...styles.modeButton,
                ...(active ? styles.modeButtonActive : null),
              }}
              onClick={() => handleModeChange(option.value)}
            >
              {option.icon}
              <span>{option.label}</span>
            </button>
          );
        })}
      </div>
    );
  };

  const renderComposer = () => {
    const isChatMode = generationMode === CANVAS_MODE_CHAT;
    const isVideoMode = generationMode === CANVAS_MODE_VIDEO;
    const isImageMode = generationMode === CANVAS_MODE_IMAGE;
    const activeModel = isVideoMode ? videoSelectedModelData : selectedModelData;
    const activeModelLabel = isChatMode
      ? chatModel || t('请选择模型')
      : activeModel
        ? getModelDisplayName(activeModel)
        : t('请选择模型');
    const activePrompt = isChatMode
      ? chatPrompt
      : isVideoMode
        ? videoPrompt
        : inspiration;
    const promptHasContent =
      activePrompt.trim().length > 0 ||
      (isChatMode && chatAttachments.length > 0);
    const submitLoading = isVideoMode ? videoGenerating : isImageMode ? generating : false;
    const submitDisabled = isChatMode
      ? !promptHasContent
      : isVideoMode
        ? videoGenerating || !canGenerateVideo
        : generating || !canGenerate;
    const placeholder = isChatMode
      ? t('输入消息...')
      : isVideoMode
        ? t('描述镜头、动作、风格、时长...')
        : t('输入提示词，生成图片或继续当前图片任务...');

    const handleComposerSubmit = () => {
      if (submitDisabled) {
        return;
      }
      if (isChatMode) {
        handleSendChatMessage();
        return;
      }
      if (isVideoMode) {
        handleGenerateVideo();
        return;
      }
      handleGenerate();
    };
    const handleComposerKeyDown = (event) => {
      const nativeEvent = event.nativeEvent || {};
      if (
        event.key !== 'Enter' ||
        event.shiftKey ||
        composerComposingRef.current ||
        nativeEvent.isComposing ||
        event.keyCode === 229
      ) {
        return;
      }
      event.preventDefault();
      handleComposerSubmit();
    };
    const renderUploadIconButton = ({
      disabled = false,
      title = t('上传图片'),
    } = {}) => (
      <div
        aria-label={title}
        title={title}
        style={{
          ...styles.uploadIconBtn,
          opacity: disabled ? 0.5 : 1,
          cursor: disabled ? 'not-allowed' : 'pointer',
        }}
      >
        <IconUpload size='small' />
      </div>
    );
    const chatComposerParameters = [
      renderPillDropdown({
        key: 'chat-model',
        label: t('模型'),
        value: chatModel,
        displayValue: chatModel,
        onChange: setChatModel,
        options: chatModels.map((model) => ({ value: model, label: model })),
        disabled: chatModels.length === 0,
      }),
      renderPillDropdown({
        key: 'chat-temperature',
        label: t('温度'),
        value: chatTemperature,
        displayValue: chatTemperature,
        onChange: setChatTemperature,
        options: ['0', '0.2', '0.7', '1', '1.5'].map((item) => ({
          value: item,
          label: item,
        })),
      }),
      renderPillDropdown({
        key: 'chat-context',
        label: t('上下文'),
        value: chatContext,
        displayValue: chatContext,
        onChange: setChatContext,
        options: ['4', '8', '16', '32'].map((item) => ({
          value: item,
          label: item,
        })),
      }),
      <label
        key='chat-tools'
        style={{
          ...styles.pillButton,
          ...(chatToolsEnabled ? styles.pillButtonActive : styles.pillButtonMuted),
          cursor: 'pointer',
        }}
      >
        <Checkbox
          checked={chatToolsEnabled}
          onChange={(event) => setChatToolsEnabled(event.target.checked)}
        />
        <span>{t('工具')}</span>
      </label>,
    ];
    const imageComposerParameters = [
      renderPillDropdown({
        key: 'image-group',
        label: t('分组'),
        value: selectedGroup,
        displayValue: selectedGroup,
        onChange: setSelectedGroup,
        options: groupOptions
          .filter((group) => group.has_available_token !== false)
          .map((group) => ({
            value: group.group,
            label: group.group,
          })),
        disabled: groupLoading || groupOptions.length === 0,
      }),
      renderModelDropdown(false, activeModelLabel),
      showImageAspectRatioSelector &&
        renderPillDropdown({
          key: 'image-aspect-ratio',
          label: t('比例'),
          value: aspectRatio,
          displayValue: aspectRatio,
          onChange: setAspectRatio,
          options: availableAspectRatios.map((ratio) => ({
            value: ratio,
            label: ratio,
          })),
        }),
      showImageResolutionSelector &&
        renderPillDropdown({
          key: 'image-resolution',
          label: t('分辨率'),
          value: resolution,
          displayValue: resolution,
          onChange: setResolution,
          options: availableResolutions.map((res) => ({
            value: res,
            label: res,
          })),
        }),
      renderPillDropdown({
        key: 'image-quantity',
        label: t('数量'),
        value: quantity,
        displayValue: String(quantity),
        onChange: (val) => setQuantity(normalizeTaskCount(val)),
        options: Array.from({ length: DEFAULT_MAX_BATCH_TASKS }, (_, index) => {
          const value = index + 1;
          return { value, label: String(value) };
        }),
      }),
      selectedModelSupportsMaskEditing &&
        referenceImages.length > 0 && (
          <button
            key='image-advanced'
            type='button'
            style={{
              ...styles.pillButton,
              ...(composerAdvancedVisible
                ? styles.pillButtonActive
                : styles.pillButtonMuted),
              cursor: 'pointer',
            }}
            onClick={() => setComposerAdvancedVisible((current) => !current)}
          >
            <IconSetting size='small' />
            <span>{t('高级')}</span>
          </button>
        ),
    ].filter(Boolean);
    const videoComposerParameters = [
      renderModelDropdown(true, activeModelLabel),
      showVideoAspectRatioSelector &&
        renderPillDropdown({
          key: 'video-aspect-ratio',
          label: t('比例'),
          value: videoAspectRatio,
          displayValue: videoAspectRatio,
          onChange: setVideoAspectRatio,
          options: videoAvailableAspectRatios.map((ratio) => ({
            value: ratio,
            label: ratio,
          })),
        }),
      showVideoResolutionSelector &&
        renderPillDropdown({
          key: 'video-resolution',
          label: t('分辨率'),
          value: videoResolution,
          displayValue: videoResolution,
          onChange: setVideoResolution,
          options: videoAvailableResolutions.map((res) => ({
            value: res,
            label: res,
          })),
        }),
      renderPillDropdown({
        key: 'video-duration',
        label: t('时长'),
        value: videoDuration,
        displayValue: videoDuration ? `${videoDuration}s` : '',
        onChange: setVideoDuration,
        options: (videoSelectedModelData?.duration_options || []).map((item) => ({
          value: item,
          label: `${item}s`,
        })),
        disabled: !videoSelectedModelData?.duration_options?.length,
      }),
    ].filter(Boolean);
    const activeParameters = isChatMode
      ? chatComposerParameters
      : isVideoMode
        ? videoComposerParameters
        : imageComposerParameters;
    const showPromptAssetBar =
      (isChatMode && chatAttachments.length > 0) ||
      (isVideoMode && videoSelectedModelSupportsImageToVideo) ||
      (isImageMode && selectedModelSupportsEditing);

    return (
      <div style={styles.composerDock}>
        <div style={styles.composerShell} data-canvas-composer={generationMode}>
          <div style={styles.promptArea}>
            {showPromptAssetBar ? (
              <div style={styles.promptAssetBar}>
                {isChatMode
                  ? chatAttachments.map((file) =>
                      renderReferenceThumb(file, () =>
                        setChatAttachments((prev) =>
                          prev.filter((item) => item.uid !== file.uid),
                        ),
                      ),
                    )
                  : null}
                {isImageMode && selectedModelSupportsEditing ? (
                  <Upload
                    action=''
                    accept='image/*'
                    multiple
                    fileList={referenceImages}
                    onChange={handleImageUpload}
                    showUploadList={false}
                    beforeUpload={validateImageSize}
                    disabled={referenceImageLimitReached}
                  >
                    {renderUploadIconButton({
                      disabled: referenceImageLimitReached,
                      title: referenceImageLimitReached
                        ? t('已达到当前模型参考图上限')
                        : t('上传图片'),
                    })}
                  </Upload>
                ) : null}
                {isImageMode && selectedModelSupportsEditing
                  ? referenceImages.map((file) =>
                      renderReferenceThumb(file, () => handleImageRemove(file)),
                    )
                  : null}
                {isVideoMode && videoSelectedModelSupportsImageToVideo ? (
                  <Upload
                    action=''
                    accept='image/*'
                    multiple={false}
                    fileList={videoReferenceImage ? [videoReferenceImage] : []}
                    onChange={handleVideoReferenceUpload}
                    showUploadList={false}
                    beforeUpload={validateImageSize}
                  >
                    {renderUploadIconButton({ title: t('上传图片') })}
                  </Upload>
                ) : null}
                {isVideoMode &&
                videoSelectedModelSupportsImageToVideo &&
                videoReferenceImage
                  ? renderReferenceThumb(videoReferenceImage, handleVideoReferenceRemove)
                  : null}
              </div>
            ) : null}
            <TextArea
              aria-label={placeholder}
              data-canvas-prompt-input={generationMode}
              placeholder={placeholder}
              value={activePrompt}
              onChange={
                isChatMode ? setChatPrompt : isVideoMode ? setVideoPrompt : setInspiration
              }
              onKeyDown={handleComposerKeyDown}
              onCompositionStart={() => {
                composerComposingRef.current = true;
              }}
              onCompositionEnd={() => {
                composerComposingRef.current = false;
              }}
              maxLength={5000}
              showClear
              borderless
              autosize={{ minRows: isMobile ? 2 : 3, maxRows: 8 }}
              style={styles.promptInput}
            />
            <div style={styles.promptInputHint}>
              <span>
                {isChatMode
                  ? activeModelLabel
                  : activeModel?.model_series
                    ? `${formatModelSeries(activeModel.model_series)} · ${activeModelLabel}`
                    : activeModelLabel}
              </span>
              <span>{t('Enter 发送，Shift + Enter 换行')}</span>
            </div>
          </div>

          <div style={styles.footer}>
            <div style={styles.footerRow}>
              <div style={styles.footerRowLeft}>{activeParameters}</div>
              <div style={styles.footerRowRight}>
                <button
                  aria-label={
                    isChatMode
                      ? t('发送消息')
                      : isVideoMode
                        ? t('生成视频')
                        : t('生成图片')
                  }
                  style={{
                    ...styles.generateIconBtn,
                    opacity: submitDisabled ? 0.55 : 1,
                    pointerEvents: submitDisabled ? 'none' : 'auto',
                    background: promptHasContent
                      ? '#f8fafc'
                      : 'rgba(15, 23, 42, 0.9)',
                    borderColor: promptHasContent
                      ? '#f8fafc'
                      : 'rgba(148, 163, 184, 0.24)',
                    color: promptHasContent ? '#020617' : '#94a3b8',
                  }}
                  onClick={handleComposerSubmit}
                  disabled={submitDisabled}
                  type='button'
                >
                  {submitLoading ? <Spin size='small' /> : <IconSend size='small' />}
                </button>
              </div>
            </div>

            {isImageMode &&
            selectedModelSupportsMaskEditing &&
            referenceImages.length > 0 &&
            composerAdvancedVisible ? (
              <div
                style={{
                  borderTop: '1px solid rgba(148, 163, 184, 0.16)',
                  paddingTop: 10,
                }}
              >
                <Text
                  size='small'
                  style={{
                    display: 'block',
                    marginBottom: 8,
                    color: 'rgba(226, 232, 240, 0.72)',
                  }}
                >
                  {t('遮罩会与第一张参考图一起作为标准编辑请求提交')}
                </Text>
                <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
                  {maskImage ? renderReferenceThumb(maskImage, handleMaskRemove) : null}
                  <Upload
                    action=''
                    accept='image/*'
                    multiple={false}
                    fileList={maskImage ? [maskImage] : []}
                    onChange={handleMaskUpload}
                    showUploadList={false}
                    beforeUpload={validateImageSize}
                    disabled={referenceImages.length === 0}
                  >
                    <div
                      style={{
                        ...styles.addImageBtn,
                        opacity: referenceImages.length === 0 ? 0.5 : 1,
                        cursor:
                          referenceImages.length === 0 ? 'not-allowed' : 'pointer',
                      }}
                    >
                      <IconSetting size='large' />
                    </div>
                  </Upload>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    );
  };

  const renderAssetCard = (asset) => {
    const imageUrl = getAssetImageUrl(asset);
    return (
      <div key={getAssetKey(asset)} style={styles.assetCard} data-canvas-asset-card='true'>
        {imageUrl ? (
          <img src={imageUrl} alt='' style={styles.assetThumb} />
        ) : (
          <div style={{ ...styles.assetThumb, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <IconImage size='large' />
          </div>
        )}
        <div style={{ padding: '8px 8px 0' }}>
          <Text size='small' ellipsis={{ rows: 2 }}>
            {asset.prompt || t('图片资产')}
          </Text>
        </div>
        <div style={styles.assetActions}>
          <Button size='small' type='primary' onClick={() => insertAssetIntoCurrentMode(asset)}>
            {generationMode === CANVAS_MODE_CHAT
              ? t('插入')
              : generationMode === CANVAS_MODE_VIDEO
                ? t('首帧')
                : t('参考图')}
          </Button>
          <Button size='small' onClick={() => setSelectedAssetPreview(asset)}>
            {t('预览')}
          </Button>
          <Button
            size='small'
            aria-label={t('下载图片')}
            icon={<IconDownload />}
            onClick={() => downloadAsset(asset)}
          />
          <Popconfirm
            title={t('确认删除该资产？')}
            content={t('删除后无法恢复，请确认是否继续')}
            okType='danger'
            onConfirm={() => deleteAsset(asset)}
          >
            <Button
              size='small'
              type='danger'
              theme='borderless'
              aria-label={t('删除资产')}
              icon={<IconDelete />}
            />
          </Popconfirm>
        </div>
      </div>
    );
  };

  const renderAssetDrawer = () => (
    <SideSheet
      visible={assetLibraryVisible}
      title={t('资产库')}
      width={isMobile ? '100%' : 560}
      onCancel={() => setAssetLibraryVisible(false)}
      bodyStyle={{ padding: isMobile ? 12 : 16 }}
      data-canvas-asset-drawer='true'
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {selectedAssetPreview ? (
          <div style={styles.detailPanel}>
            <div style={styles.detailHeader}>
              <Text strong>{t('资产预览')}</Text>
              <Button size='small' onClick={() => setSelectedAssetPreview(null)}>
                {t('收起')}
              </Button>
            </div>
            <div style={{ padding: 12 }}>
              <img
                src={getAssetImageUrl(selectedAssetPreview)}
                alt=''
                style={{ width: '100%', maxHeight: 360, objectFit: 'contain' }}
              />
              <Text type='tertiary' size='small' style={{ display: 'block', marginTop: 8 }}>
                {selectedAssetPreview.prompt || t('暂无提示词')}
              </Text>
            </div>
          </div>
        ) : null}
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
          <Text type='tertiary' size='small'>
            {t('资产总数')}：{canvasAssetsTotal}
          </Text>
          <Button
            size='small'
            icon={<IconRefresh />}
            onClick={() => loadCanvasAssets(canvasAssetPage || 1)}
          >
            {t('刷新')}
          </Button>
        </div>
        <Spin spinning={canvasAssetsLoading}>
          {canvasAssetsError ? (
            renderSidebarEmpty(
              t('加载资产失败'),
              canvasAssetsError,
              <Button size='small' onClick={() => loadCanvasAssets(1)}>
                {t('重试')}
              </Button>,
            )
          ) : canvasAssets.length === 0 ? (
            renderSidebarEmpty(t('暂无图片资产'), t('完成图片生成后会出现在这里'))
          ) : (
            <div style={styles.assetGrid}>{canvasAssets.map(renderAssetCard)}</div>
          )}
        </Spin>
        <div style={styles.drawerFooterPager}>
          <Button
            size='small'
            disabled={canvasAssetPage <= 1 || canvasAssetsLoading}
            onClick={() => loadCanvasAssets(Math.max(1, canvasAssetPage - 1))}
          >
            {t('上一页')}
          </Button>
          <Text type='tertiary' size='small'>
            {t('第 {{page}} 页', { page: canvasAssetPage })}
          </Text>
          <Button
            size='small'
            disabled={
              canvasAssetsLoading ||
              canvasAssetPage * DEFAULT_ASSET_PAGE_SIZE >= canvasAssetsTotal
            }
            onClick={() => loadCanvasAssets(canvasAssetPage + 1)}
          >
            {t('下一页')}
          </Button>
        </div>
      </div>
    </SideSheet>
  );

  const renderWorkspace = () => (
    <div style={styles.rightPanel}>
      <div style={styles.workspaceTopbar}>
        <div style={styles.workspaceTopbarLeft}>
          {isMobile ? (
            <Button
              type='tertiary'
              aria-label={t('打开任务栏')}
              data-canvas-mobile-taskbar-trigger='true'
              icon={<IconMenu />}
              onClick={() => setMobileTaskbarVisible(true)}
            />
          ) : null}
          {renderModeSwitch()}
        </div>
        <Button
          icon={<IconArchive />}
          type='tertiary'
          aria-label={t('打开资产库')}
          data-canvas-asset-library-trigger='true'
          onClick={() => setAssetLibraryVisible(true)}
        >
          {t('资产库')}
        </Button>
      </div>
      <div style={styles.workspaceBody}>
        {renderMainContent()}
        {renderComposer()}
      </div>
    </div>
  );

  return (
    <div style={styles.container}>
      {!isMobile ? renderTaskSidebar() : null}
      {renderWorkspace()}
      {isMobile ? (
        <SideSheet
          visible={mobileTaskbarVisible}
          title={currentTaskSidebarTitle}
          width='100%'
          onCancel={() => setMobileTaskbarVisible(false)}
          bodyStyle={{ padding: 0 }}
        >
          {renderTaskSidebar()}
        </SideSheet>
      ) : null}
      {renderAssetDrawer()}
      <ImageGenerationTaskModal
        visible={taskModalVisible}
        task={selectedTask}
        onClose={() => setTaskModalVisible(false)}
        onRetrySuccess={(task) => {
          updateTaskInList(task);
          setSelectedTask(task);
        }}
        onDeleted={(taskId) => {
          setTasks((prev) => prev.filter((task) => task.id !== taskId));
          setSelectedTask(null);
          setTaskModalVisible(false);
        }}
      />
      <VideoGenerationTaskModal
        visible={videoTaskModalVisible}
        task={videoSelectedTask}
        onClose={() => setVideoTaskModalVisible(false)}
        onRetrySuccess={(task) => {
          updateVideoTaskInList(task);
          setVideoSelectedTask(task);
        }}
        onDeleted={(taskId) => {
          setVideoTasks((prev) => prev.filter((task) => task.id !== taskId));
          setVideoSelectedTask(null);
          setVideoTaskModalVisible(false);
        }}
      />
    </div>
  );
};

export default ImageGeneration;
