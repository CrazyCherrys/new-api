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
  TextArea,
  SideSheet,
  Empty,
  Checkbox,
  Tag,
  Popconfirm,
  Tooltip,
} from '@douyinfe/semi-ui';
import {
  IconDelete,
  IconImage,
  IconChevronLeft,
  IconChevronDown,
  IconSend,
  IconMenu,
  IconVideo,
  IconSetting,
  IconUpload,
  IconCommentStroked,
  IconArchive,
  IconRefresh,
  IconDownload,
  IconClock,
  IconLayers,
  IconPlayCircle,
  IconRealSizeStroked,
  IconText,
  IconExternalOpen,
  IconMore,
  IconSidebar,
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
import {
  CANVAS_RENDERABLE_IMAGE_BATCH,
  getRenderableCanvasMessages,
} from './canvasMessageBatches';

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

const normalizeComparableId = (value) => {
  if (value === undefined || value === null) {
    return '';
  }
  return String(value);
};

const buildComparableIdSet = (values) => {
  const list =
    values instanceof Set ? Array.from(values) : [].concat(values || []);
  return new Set(
    list
      .map(normalizeComparableId)
      .filter((value) => value !== ''),
  );
};

const parseCanvasTaskParams = (params) => {
  if (!params) {
    return {};
  }
  if (typeof params === 'object' && !Array.isArray(params)) {
    return params;
  }
  if (typeof params !== 'string') {
    return {};
  }
  try {
    const parsed = JSON.parse(params);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
};

const getCanvasMessageTaskType = (message) =>
  message?.task_type ||
  (message?.video_task
    ? 'video_generation'
    : message?.image_task
      ? 'image_generation'
      : '');

const getCanvasMessagePromptText = (message) =>
  String(
    message?.prompt ||
      message?.image_task?.prompt ||
      message?.video_task?.prompt ||
      '',
  ).trim();

const normalizeAspectRatioText = (value) => {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  const match = text.match(
    /^(\d+(?:\.\d+)?)\s*[:/xX×]\s*(\d+(?:\.\d+)?)$/,
  );
  if (!match) {
    return '';
  }
  return `${match[1]} / ${match[2]}`;
};

const buildAspectRatioTextFromDimensions = (width, height) => {
  const normalizedWidth = Number(width);
  const normalizedHeight = Number(height);
  if (
    !Number.isFinite(normalizedWidth) ||
    !Number.isFinite(normalizedHeight) ||
    normalizedWidth <= 0 ||
    normalizedHeight <= 0
  ) {
    return '';
  }
  return `${Math.round(normalizedWidth)} / ${Math.round(normalizedHeight)}`;
};

const normalizeImageResolutionTier = (value) => {
  const text = String(value || '').trim().toUpperCase();
  return ['1K', '2K', '4K'].includes(text) ? text : '';
};

const normalizeImageAspectRatioValue = (value) => {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }
  if (text.toLowerCase() === 'auto') {
    return 'auto';
  }
  return text
    .toLowerCase()
    .replace(/×/g, 'x')
    .replace(/\s+/g, '')
    .replace(/x/g, ':');
};

const roundImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.round(value / multiple) * multiple);

const floorImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.floor(value / multiple) * multiple);

const ceilImageSizeToMultiple = (value, multiple = 16) =>
  Math.max(multiple, Math.ceil(value / multiple) * multiple);

const normalizeImagePreviewDimensions = (rawWidth, rawHeight) => {
  const multiple = 16;
  const maxEdge = 3840;
  const maxAspect = 3;
  const minPixels = 655360;
  const maxPixels = 8294400;
  let width = roundImageSizeToMultiple(rawWidth, multiple);
  let height = roundImageSizeToMultiple(rawHeight, multiple);

  const scaleToFit = (scale) => {
    width = floorImageSizeToMultiple(width * scale, multiple);
    height = floorImageSizeToMultiple(height * scale, multiple);
  };
  const scaleToFill = (scale) => {
    width = ceilImageSizeToMultiple(width * scale, multiple);
    height = ceilImageSizeToMultiple(height * scale, multiple);
  };

  for (let index = 0; index < 4; index += 1) {
    const edge = Math.max(width, height);
    if (edge > maxEdge) {
      scaleToFit(maxEdge / edge);
    }

    if (width / height > maxAspect) {
      width = floorImageSizeToMultiple(height * maxAspect, multiple);
    } else if (height / width > maxAspect) {
      height = floorImageSizeToMultiple(width * maxAspect, multiple);
    }

    const pixels = width * height;
    if (pixels > maxPixels) {
      scaleToFit(Math.sqrt(maxPixels / pixels));
    } else if (pixels < minPixels) {
      scaleToFill(Math.sqrt(minPixels / pixels));
    }
  }

  return { width, height };
};

const parseImageAspectRatioPair = (ratio) => {
  const match = String(ratio || '')
    .trim()
    .match(/^(\d+(?:\.\d+)?)\s*[:xX×]\s*(\d+(?:\.\d+)?)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return null;
  }
  return { width, height };
};

const parseImageSizePair = (size) => {
  const match = String(size || '')
    .trim()
    .match(/^(\d+)\s*[xX×]\s*(\d+)$/);
  if (!match) {
    return null;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return null;
  }
  return { width, height };
};

const getImageDimensionPreview = (resolutionValue, aspectRatioValue) => {
  const explicitSize = parseImageSizePair(resolutionValue);
  if (explicitSize) {
    return {
      width: String(explicitSize.width),
      height: String(explicitSize.height),
    };
  }

  const normalizedRatio = normalizeImageAspectRatioValue(aspectRatioValue);
  if (normalizedRatio === 'auto') {
    return { width: 'auto', height: 'auto' };
  }

  const tier = normalizeImageResolutionTier(resolutionValue);
  if (!tier) {
    return { width: 'auto', height: 'auto' };
  }

  const ratio = parseImageAspectRatioPair(normalizedRatio || '1:1');
  if (!ratio) {
    return { width: 'auto', height: 'auto' };
  }

  const isSquare = ratio.width === ratio.height;
  if (isSquare) {
    const side = tier === '4K' ? 3840 : tier === '2K' ? 2048 : 1024;
    const normalized = normalizeImagePreviewDimensions(side, side);
    return {
      width: String(normalized.width),
      height: String(normalized.height),
    };
  }

  if (tier === '1K') {
    const shortSide = 1024;
    if (ratio.width > ratio.height) {
      return {
        width: String(roundImageSizeToMultiple((shortSide * ratio.width) / ratio.height)),
        height: String(shortSide),
      };
    }
    return {
      width: String(shortSide),
      height: String(roundImageSizeToMultiple((shortSide * ratio.height) / ratio.width)),
    };
  }

  const longSide = tier === '4K' ? 3840 : 2048;
  let rawWidth = longSide;
  let rawHeight = longSide;
  if (ratio.width > ratio.height) {
    rawHeight = roundImageSizeToMultiple((longSide * ratio.height) / ratio.width);
  } else {
    rawWidth = roundImageSizeToMultiple((longSide * ratio.width) / ratio.height);
  }
  const normalized = normalizeImagePreviewDimensions(rawWidth, rawHeight);
  return {
    width: String(normalized.width),
    height: String(normalized.height),
  };
};

const collectTaskReferenceValues = (raw) => {
  const values = [];
  const append = (item) => {
    if (!item) {
      return;
    }
    if (typeof item === 'string') {
      const trimmed = item.trim();
      if (trimmed) {
        values.push(trimmed);
      }
      return;
    }
    if (typeof item === 'object') {
      append(item.url || item.image_url || item.thumbnail_url);
    }
  };

  if (Array.isArray(raw)) {
    raw.forEach(append);
  } else {
    append(raw);
  }

  return Array.from(new Set(values));
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
  const [generationMode, setGenerationMode] = useState(CANVAS_MODE_CHAT);
  const [models, setModels] = useState([]);
  const [filteredModels, setFilteredModels] = useState([]);
  const [videoModels, setVideoModels] = useState([]);
  const [videoFilteredModels, setVideoFilteredModels] = useState([]);
  const [mobileTaskbarVisible, setMobileTaskbarVisible] = useState(false);
  const [desktopSidebarCollapsed, setDesktopSidebarCollapsed] = useState(false);
  const [composerAdvancedVisible, setComposerAdvancedVisible] = useState(false);
  const [activeDropdownKey, setActiveDropdownKey] = useState('');
  const [assetLibraryVisible, setAssetLibraryVisible] = useState(false);
  const [canvasAssets, setCanvasAssets] = useState([]);
  const [canvasAssetsLoading, setCanvasAssetsLoading] = useState(false);
  const [canvasAssetsError, setCanvasAssetsError] = useState('');
  const [canvasAssetsTotal, setCanvasAssetsTotal] = useState(0);
  const [canvasAssetPage, setCanvasAssetPage] = useState(1);
  const [selectedAssetPreview, setSelectedAssetPreview] = useState(null);
  const [selectedCanvasImagePreview, setSelectedCanvasImagePreview] = useState(null);
  const [selectedCanvasAssetIds, setSelectedCanvasAssetIds] = useState(
    new Set(),
  );
  const [assetBatchDeleting, setAssetBatchDeleting] = useState(false);

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
  const [canvasSessions, setCanvasSessions] = useState({
    [CANVAS_MODE_CHAT]: [],
    [CANVAS_MODE_IMAGE]: [],
    [CANVAS_MODE_VIDEO]: [],
  });
  const [canvasSessionsLoading, setCanvasSessionsLoading] = useState({
    [CANVAS_MODE_CHAT]: false,
    [CANVAS_MODE_IMAGE]: false,
    [CANVAS_MODE_VIDEO]: false,
  });
  const [canvasSessionErrors, setCanvasSessionErrors] = useState({
    [CANVAS_MODE_CHAT]: '',
    [CANVAS_MODE_IMAGE]: '',
    [CANVAS_MODE_VIDEO]: '',
  });
  const [selectedCanvasSessionIds, setSelectedCanvasSessionIds] = useState({
    [CANVAS_MODE_CHAT]: null,
    [CANVAS_MODE_IMAGE]: null,
    [CANVAS_MODE_VIDEO]: null,
  });
  const [deletingCanvasSession, setDeletingCanvasSession] = useState(false);
  const [canvasMessagesSessionId, setCanvasMessagesSessionId] = useState(null);
  const [canvasMessages, setCanvasMessages] = useState([]);
  const [canvasMessagesLoading, setCanvasMessagesLoading] = useState(false);
  const [canvasMessagesError, setCanvasMessagesError] = useState('');

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
  const [selectedCanvasMessageId, setSelectedCanvasMessageId] = useState(null);
  const canvasMessageViewportRef = useRef(null);
  const canvasMessageDetailCacheRef = useRef(new Map());
  const canvasMessageDetailRequestSeqRef = useRef(new Map());
  const canvasMessageHydrationAttemptRef = useRef(new Set());
  const canvasMessageClientRequestSeqRef = useRef(0);
  const sseRef = useRef(null);
  const pollingTimerRef = useRef(null);
  const pollingIntervalRef = useRef(DEFAULT_POLLING_INTERVAL_SECONDS);
  const taskListStateRef = useRef(null);
  const taskListRequestSeqRef = useRef(0);
  const taskDetailRequestSeqRef = useRef(0);
  const drawingModelsRequestSeqRef = useRef(0);
  const loadedModelsGroupRef = useRef('');
  const canvasSessionsRequestSeqRef = useRef({
    [CANVAS_MODE_CHAT]: 0,
    [CANVAS_MODE_IMAGE]: 0,
    [CANVAS_MODE_VIDEO]: 0,
  });
  const canvasMessagesRequestSeqRef = useRef(0);
  const canvasMessagesSessionIdRef = useRef(null);
  const taskUpdatesCompletedSinceRef = useRef(
    Math.floor(Date.now() / 1000) - 60,
  );
  const taskCursorHistoryRef = useRef(['']);
  const pendingCanvasPrefillRef = useRef(null);
  const prefillGroupFallbackNoticeShownRef = useRef(false);
  const composerComposingRef = useRef(false);
  const blankCanvasSelectionModesRef = useRef({});
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
  const currentCanvasSessions = canvasSessions[generationMode] || [];
  const selectedCanvasSessionId = selectedCanvasSessionIds[generationMode];
  const selectedCanvasSession = currentCanvasSessions.find(
    (session) => session.id === selectedCanvasSessionId,
  );
  const unifiedCanvasSessions = useMemo(
    () =>
      CANVAS_MODES.flatMap((mode) =>
        (canvasSessions[mode] || []).map((session) => ({
          ...session,
          mode: session.mode || mode,
        })),
      ).sort((a, b) => {
        if (!!a.pinned !== !!b.pinned) {
          return a.pinned ? -1 : 1;
        }
        return (Number(b.updated_time) || 0) - (Number(a.updated_time) || 0);
      }),
    [canvasSessions],
  );
  const unifiedCanvasSessionsLoading = CANVAS_MODES.some(
    (mode) => canvasSessionsLoading[mode],
  );
  const unifiedCanvasSessionError = CANVAS_MODES.map(
    (mode) => canvasSessionErrors[mode],
  ).find(Boolean);
  const displayedCanvasMessages =
    canvasMessagesSessionId === selectedCanvasSessionId ? canvasMessages : [];
  const isCurrentCanvasMessageSession = (sessionId) =>
    String(canvasMessagesSessionIdRef.current || '') === String(sessionId || '');
  const updateCurrentCanvasSessionModel = async (mode, modelId) => {
    const normalizedMode = CANVAS_MODES.includes(mode) ? mode : generationMode;
    const sessionId = selectedCanvasSessionIds[normalizedMode];
    const session = (canvasSessions[normalizedMode] || []).find(
      (item) => item.id === sessionId,
    );
    if (!session?.id) {
      return;
    }
    const currentModel = String(modelId || '').trim();
    setCanvasSessionsForMode(normalizedMode, (prev) =>
      prev.map((item) =>
        item.id === session.id ? { ...item, current_model: currentModel } : item,
      ),
    );
    try {
      const res = await API.patch(`/api/canvas/sessions/${session.id}`, {
        current_model: currentModel,
      });
      if (res.data.success && res.data.data) {
        updateCanvasSessionInState(res.data.data);
      } else {
        showError(res.data.message || t('更新会话模型失败'));
      }
    } catch (error) {
      showError(error.message || t('更新会话模型失败'));
    }
  };

  useEffect(() => {
    const container = canvasMessageViewportRef.current;
    if (!container) {
      return;
    }
    container.scrollTop = container.scrollHeight;
  }, [canvasMessagesSessionId, displayedCanvasMessages.length, generationMode]);

  useEffect(() => {
    setSelectedCanvasMessageId(null);
  }, [selectedCanvasSessionId, generationMode]);

  useEffect(() => {
    if (
      generationMode === CANVAS_MODE_CHAT ||
      displayedCanvasMessages.length === 0
    ) {
      return;
    }
    displayedCanvasMessages.forEach((message) => {
      if (message?.role === 'user' || !message?.task_id) {
        return;
      }
      const taskType = getCanvasMessageTaskType(message);
      const task = message.image_task || message.video_task || null;
      const taskId = String(task?.id || message.task_id || '');
      if (message?.client_request_id || taskId.startsWith('pending-')) {
        return;
      }
      if (
        (task?.params || task?.request_params) &&
        (message.reference_images || message.reference_image)
      ) {
        return;
      }
      const cacheKey = `${taskType}:${taskId}`;
      if (canvasMessageHydrationAttemptRef.current.has(cacheKey)) {
        return;
      }
      canvasMessageHydrationAttemptRef.current.add(cacheKey);
      loadCanvasMessageTaskDetail(message).then((detail) => {
        if (!detail) {
          return;
        }
        const mergedMessage = mergeCanvasMessageTaskDetail(message, detail);
        if (taskType === 'video_generation') {
          setCanvasMessages((prev) =>
            prev.map((item) =>
              item.id === message.id
                ? {
                    ...mergedMessage,
                    canvas_aspect_ratio:
                      mergedMessage.canvas_aspect_ratio ||
                      getCanvasMessageAspectRatio(mergedMessage),
                  }
                : item,
            ),
          );
          return;
        }
        setCanvasMessages((prev) =>
          prev.map((item) =>
            item.id === message.id
              ? {
                  ...mergedMessage,
                  canvas_aspect_ratio:
                    mergedMessage.canvas_aspect_ratio ||
                    getCanvasMessageAspectRatio(mergedMessage),
                }
              : item,
          ),
        );
      });
    });
  }, [displayedCanvasMessages, generationMode]);

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

  const taskMatchesTimeFilter = (task, timeFilter) => {
    if (!timeFilter) {
      return true;
    }
    const createdTime = Number(task?.created_time) || 0;
    if (createdTime <= 0) {
      return false;
    }
    const { start, end } = computeTimeRange(timeFilter);
    return start > 0 && createdTime >= start && (!end || createdTime <= end);
  };

  const imageTaskMatchesCurrentTaskFilters = (task) => {
    if (!task) {
      return false;
    }
    if (taskStatusFilter && task.status !== taskStatusFilter) {
      return false;
    }
    if (taskModelFilter && task.model_id !== taskModelFilter) {
      return false;
    }
    return taskMatchesTimeFilter(task, taskTimeFilter);
  };

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
      setGenerationMode(CANVAS_MODE_IMAGE);
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
          setGenerationMode(CANVAS_MODE_IMAGE);
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

  useEffect(() => {
    CANVAS_MODES.forEach((mode) => {
      loadCanvasSessions(mode);
    });
  }, []);

  useEffect(() => {
    if (!selectedCanvasSessionId) {
      canvasMessagesRequestSeqRef.current += 1;
      canvasMessagesSessionIdRef.current = null;
      setCanvasMessagesSessionId(null);
      setCanvasMessages([]);
      setCanvasMessagesError('');
      setCanvasMessagesLoading(false);
      return;
    }
    loadCanvasMessages(selectedCanvasSessionId);
  }, [selectedCanvasSessionId]);

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
      setSelectedCanvasAssetIds(new Set());
    } catch (error) {
      const message = error.message || t('加载资产失败');
      setCanvasAssetsError(message);
      showError(message);
    } finally {
      setCanvasAssetsLoading(false);
    }
  };

  const setCanvasSessionsForMode = (mode, updater) => {
    setCanvasSessions((prev) => ({
      ...prev,
      [mode]:
        typeof updater === 'function'
          ? updater(prev[mode] || [])
          : updater || [],
    }));
  };

  const setCanvasSessionsLoadingForMode = (mode, loading) => {
    setCanvasSessionsLoading((prev) => ({
      ...prev,
      [mode]: loading,
    }));
  };

  const setCanvasSessionErrorForMode = (mode, message) => {
    setCanvasSessionErrors((prev) => ({
      ...prev,
      [mode]: message || '',
    }));
  };

  const loadCanvasSessions = async (mode = generationMode, options = {}) => {
    const normalizedMode = CANVAS_MODES.includes(mode) ? mode : CANVAS_MODE_IMAGE;
    const requestSeq =
      (canvasSessionsRequestSeqRef.current[normalizedMode] || 0) + 1;
    canvasSessionsRequestSeqRef.current[normalizedMode] = requestSeq;
    if (!options.silent) {
      setCanvasSessionsLoadingForMode(normalizedMode, true);
    }
    try {
      const res = await API.get('/api/canvas/sessions', {
        params: { mode: normalizedMode },
      });
      if (requestSeq !== canvasSessionsRequestSeqRef.current[normalizedMode]) {
        return [];
      }
      if (!res.data.success) {
        const message = res.data.message || t('加载会话失败');
        setCanvasSessionErrorForMode(normalizedMode, message);
        if (!options.silent) {
          showError(message);
        }
        return [];
      }
      const sessions = (res.data.data || [])
        .map((session) => ({
          ...session,
          mode: session.mode || normalizedMode,
        }))
        .filter((session) => session.mode === normalizedMode);
      setCanvasSessionErrorForMode(normalizedMode, '');
      setCanvasSessionsForMode(normalizedMode, sessions);
      setSelectedCanvasSessionIds((prev) => {
        const currentId = prev[normalizedMode];
        if (currentId && sessions.some((session) => session.id === currentId)) {
          return prev;
        }
        if (blankCanvasSelectionModesRef.current[normalizedMode]) {
          return {
            ...prev,
            [normalizedMode]: null,
          };
        }
        return {
          ...prev,
          [normalizedMode]: sessions[0]?.id || null,
        };
      });
      return sessions;
    } catch (error) {
      if (requestSeq !== canvasSessionsRequestSeqRef.current[normalizedMode]) {
        return [];
      }
      const message = error.message || t('加载会话失败');
      setCanvasSessionErrorForMode(normalizedMode, message);
      if (!options.silent) {
        showError(message);
      }
      return [];
    } finally {
      if (
        !options.silent &&
        requestSeq === canvasSessionsRequestSeqRef.current[normalizedMode]
      ) {
        setCanvasSessionsLoadingForMode(normalizedMode, false);
      }
    }
  };

  const createCanvasSession = async (mode = generationMode, title = '') => {
    const normalizedMode = CANVAS_MODES.includes(mode) ? mode : generationMode;
    canvasSessionsRequestSeqRef.current[normalizedMode] =
      (canvasSessionsRequestSeqRef.current[normalizedMode] || 0) + 1;
    setCanvasSessionsLoadingForMode(normalizedMode, false);
    const res = await API.post('/api/canvas/sessions', {
      mode: normalizedMode,
      title,
    });
    if (!res.data.success) {
      throw new Error(res.data.message || t('创建会话失败'));
    }
    const session = res.data.data;
    blankCanvasSelectionModesRef.current[normalizedMode] = false;
    canvasSessionsRequestSeqRef.current[normalizedMode] =
      (canvasSessionsRequestSeqRef.current[normalizedMode] || 0) + 1;
    setCanvasSessionErrorForMode(normalizedMode, '');
    setCanvasSessionsForMode(normalizedMode, (prev) => [
      session,
      ...prev.filter((item) => item.id !== session.id),
    ]);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [normalizedMode]: session.id,
    }));
    canvasMessagesRequestSeqRef.current += 1;
    canvasMessagesSessionIdRef.current = session.id;
    setCanvasMessagesSessionId(session.id);
    setCanvasMessages([]);
    setCanvasMessagesError('');
    setCanvasMessagesLoading(false);
    setMobileTaskbarVisible(false);
    return session;
  };

  const ensureCanvasSession = async (mode = generationMode) => {
    const sessions = canvasSessions[mode] || [];
    const existingId = selectedCanvasSessionIds[mode];
    const existing = sessions.find(
      (session) => session.id === existingId,
    );
    if (existing) {
      return existing;
    }
    return createCanvasSession(mode);
  };

  const loadCanvasMessages = async (sessionId, options = {}) => {
    if (!sessionId) {
      canvasMessagesRequestSeqRef.current += 1;
      canvasMessagesSessionIdRef.current = null;
      setCanvasMessagesSessionId(null);
      setCanvasMessages([]);
      return [];
    }
    const requestSeq = canvasMessagesRequestSeqRef.current + 1;
    canvasMessagesRequestSeqRef.current = requestSeq;
    canvasMessagesSessionIdRef.current = sessionId;
    setCanvasMessagesSessionId(sessionId);
    if (!options.silent) {
      setCanvasMessagesLoading(true);
    }
    try {
      const res = await API.get(`/api/canvas/sessions/${sessionId}/messages`);
      if (
        requestSeq !== canvasMessagesRequestSeqRef.current ||
        !isCurrentCanvasMessageSession(sessionId)
      ) {
        return [];
      }
      if (!res.data.success) {
        const message = res.data.message || t('加载消息失败');
        setCanvasMessagesError(message);
        if (!options.silent) {
          showError(message);
        }
        return [];
      }
      const messages = res.data.data || [];
      setCanvasMessages(messages);
      setCanvasMessagesError('');
      syncSelectedTaskFromCanvasMessages(messages);
      return messages;
    } catch (error) {
      if (
        requestSeq !== canvasMessagesRequestSeqRef.current ||
        !isCurrentCanvasMessageSession(sessionId)
      ) {
        return [];
      }
      const message = error.message || t('加载消息失败');
      setCanvasMessagesError(message);
      if (!options.silent) {
        showError(message);
      }
      return [];
    } finally {
      if (
        !options.silent &&
        requestSeq === canvasMessagesRequestSeqRef.current &&
        isCurrentCanvasMessageSession(sessionId)
      ) {
        setCanvasMessagesLoading(false);
      }
    }
  };

  const appendCanvasMessagesForSession = (sessionId, messages) => {
    if (!sessionId || !messages?.length || !isCurrentCanvasMessageSession(sessionId)) {
      return;
    }
    setCanvasMessages((prev) => [...prev, ...messages]);
  };

  const replaceCanvasMessagesForSession = (
    sessionId,
    requestId,
    messages,
    options = {},
  ) => {
    if (
      !sessionId ||
      !requestId ||
      !messages?.length ||
      !isCurrentCanvasMessageSession(sessionId)
    ) {
      return;
    }
    const nextMessages = messages.map((message) => ({
      ...message,
      client_request_id: requestId,
      canvas_aspect_ratio:
        message.canvas_aspect_ratio || options.canvasAspectRatio || '',
    }));
    setCanvasMessages((prev) => {
      const insertionIndex = prev.findIndex(
        (message) => message?.client_request_id === requestId,
      );
      const filtered = prev.filter(
        (message) => message?.client_request_id !== requestId,
      );
      if (insertionIndex < 0) {
        return [...filtered, ...nextMessages];
      }
      const before = filtered.slice(0, insertionIndex);
      const after = filtered.slice(insertionIndex);
      return [...before, ...nextMessages, ...after];
    });
  };

  const generateCanvasClientRequestId = () => {
    canvasMessageClientRequestSeqRef.current += 1;
    return `canvas-client-${Date.now()}-${canvasMessageClientRequestSeqRef.current}`;
  };

  const removeCanvasMessagesForRequest = (sessionId, requestId) => {
    if (!sessionId || !requestId || !isCurrentCanvasMessageSession(sessionId)) {
      return;
    }
    setCanvasMessages((prev) =>
      prev.filter((message) => message?.client_request_id !== requestId),
    );
  };

  const updateCanvasSessionInState = (session) => {
    if (!session?.mode) {
      return;
    }
    setCanvasSessionsForMode(session.mode, (prev) => {
      const next = [session, ...prev.filter((item) => item.id !== session.id)];
      return next.sort((a, b) => {
        if (a.pinned !== b.pinned) {
          return a.pinned ? -1 : 1;
        }
        return (Number(b.updated_time) || 0) - (Number(a.updated_time) || 0);
      });
    });
  };

  const renameCanvasSession = async (session) => {
    const currentTitle = session?.title || '';
    const nextTitle = window.prompt(t('重命名会话'), currentTitle);
    if (nextTitle === null) {
      return;
    }
    const trimmedTitle = nextTitle.trim();
    if (!trimmedTitle) {
      showError(t('标题不能为空'));
      return;
    }
    try {
      const res = await API.patch(`/api/canvas/sessions/${session.id}`, {
        title: trimmedTitle,
      });
      if (!res.data.success) {
        showError(res.data.message || t('重命名失败'));
        return;
      }
      updateCanvasSessionInState(res.data.data);
    } catch (error) {
      showError(error.message || t('重命名失败'));
    }
  };

  const toggleCanvasSessionPin = async (session) => {
    try {
      const res = await API.patch(`/api/canvas/sessions/${session.id}`, {
        pinned: !session.pinned,
      });
      if (!res.data.success) {
        showError(res.data.message || t('更新置顶失败'));
        return;
      }
      updateCanvasSessionInState(res.data.data);
    } catch (error) {
      showError(error.message || t('更新置顶失败'));
    }
  };

  const deleteCanvasSession = async (session) => {
    if (!session?.id) {
      return;
    }
    setDeletingCanvasSession(true);
    try {
      const res = await API.delete(`/api/canvas/sessions/${session.id}`);
      if (!res.data.success) {
        showError(res.data.message || t('删除会话失败'));
        return;
      }
      showSuccess(t('删除成功'));
      setCanvasSessionsForMode(session.mode, (prev) =>
        prev.filter((item) => item.id !== session.id),
      );
      setSelectedCanvasSessionIds((prev) => {
        if (prev[session.mode] !== session.id) {
          return prev;
        }
        const nextSessions = (canvasSessions[session.mode] || []).filter(
          (item) => item.id !== session.id,
        );
        return {
          ...prev,
          [session.mode]: nextSessions[0]?.id || null,
        };
      });
      if (isCurrentCanvasMessageSession(session.id)) {
        canvasMessagesRequestSeqRef.current += 1;
        canvasMessagesSessionIdRef.current = null;
        setCanvasMessagesSessionId(null);
        setCanvasMessages([]);
        setCanvasMessagesError('');
        setCanvasMessagesLoading(false);
      }
      if (session.mode === CANVAS_MODE_IMAGE) {
        loadTasks(true, { forceRefresh: true });
      }
      if (session.mode === CANVAS_MODE_VIDEO) {
        loadVideoTasks();
      }
    } catch (error) {
      showError(error.message || t('删除会话失败'));
    } finally {
      setDeletingCanvasSession(false);
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
    updateCurrentCanvasSessionModel(CANVAS_MODE_IMAGE, model.request_model);
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
    updateCurrentCanvasSessionModel(CANVAS_MODE_VIDEO, model.request_model);
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
        newItems.forEach((task) =>
          updateCanvasMessageTask(task, CANVAS_MODE_IMAGE),
        );
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
        newItems.forEach((task) =>
          updateCanvasMessageTask(task, CANVAS_MODE_VIDEO),
        );
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
    updates.forEach((task) => updateCanvasMessageTask(task, CANVAS_MODE_IMAGE));
  };

  const updateCanvasMessageTask = (updatedTask, mode) => {
    if (!updatedTask?.id) {
      return;
    }
    setCanvasMessages((prevMessages) =>
      prevMessages.map((message) => {
        if (!message?.task_id || String(message.task_id) !== String(updatedTask.id)) {
          return message;
        }
        const messageTaskType = getCanvasMessageTaskType(message);
        if (mode === CANVAS_MODE_VIDEO && messageTaskType !== 'video_generation') {
          return message;
        }
        if (mode === CANVAS_MODE_IMAGE && messageTaskType !== 'image_generation') {
          return message;
        }
        if (mode === CANVAS_MODE_VIDEO) {
          return {
            ...message,
            status: updatedTask.status,
            error_message:
              updatedTask.error_message ||
              updatedTask.fail_reason ||
              message.error_message,
            video_task: {
              ...(message.video_task || {}),
              ...updatedTask,
            },
          };
        }
        return {
          ...message,
          status: updatedTask.status,
          error_message:
            updatedTask.error_message ||
            updatedTask.fail_reason ||
            message.error_message,
          image_task: {
            ...(message.image_task || {}),
            ...updatedTask,
          },
        };
      }),
    );
  };

  const syncSelectedTaskFromCanvasMessages = (messages) => {
    const taskMessages = (messages || []).filter((message) => message?.task_id);
    const lastTaskMessage = taskMessages[taskMessages.length - 1];
    if (!lastTaskMessage) {
      return;
    }
    const taskType = getCanvasMessageTaskType(lastTaskMessage);
    if (taskType === 'image_generation' && lastTaskMessage.image_task) {
      setSelectedTask(lastTaskMessage.image_task);
    }
    if (taskType === 'video_generation' && lastTaskMessage.video_task) {
      setVideoSelectedTask(lastTaskMessage.video_task);
    }
  };

  const getCanvasMessageReferenceFiles = (message) => {
    if (!message) {
      return [];
    }
    const taskType = getCanvasMessageTaskType(message);
    const task = message.image_task || message.video_task || null;
    const params = parseCanvasTaskParams(task?.params || task?.request_params);
    const references = [];
    const seen = new Set();
    const addFiles = (values, prefix) => {
      collectTaskReferenceValues(values).forEach((url, index) => {
        if (seen.has(url)) {
          return;
        }
        seen.add(url);
        references.push({
          uid: `${prefix}-${message.id || task?.id || 'message'}-${index}`,
          name: `${prefix}-${index + 1}`,
          url,
        });
      });
    };

    addFiles(message.reference_images || message.reference_image, 'message-reference');
    addFiles(params.reference_images || params.reference_image, 'task-reference');
    addFiles(
      task?.reference_images || task?.reference_image,
      taskType === 'video_generation' ? 'video-reference' : 'task-reference',
    );
    return references;
  };

  const getCanvasMessageMedia = (message) => {
    if (!message) {
      return null;
    }
    const taskType = getCanvasMessageTaskType(message);
    if (taskType === 'image_generation') {
      const task = message.image_task || {};
      return {
        kind: 'image',
        status: task.status || message.status,
        src: task.thumbnail_url || task.image_url || '',
        previewSrc: task.image_url || task.thumbnail_url || '',
        error:
          task.error_message ||
          task.fail_reason ||
          message.error_message ||
          '',
      };
    }
    if (taskType === 'video_generation') {
      const task = message.video_task || {};
      return {
        kind: 'video',
        status: task.status || message.status,
        src: task.thumbnail_url || task.video_url || task.result_url || '',
        videoUrl: task.video_url || task.result_url || '',
        error:
          task.error_message ||
          task.fail_reason ||
          message.error_message ||
          '',
      };
    }
    return null;
  };

  const getCanvasMessageExplicitAspectRatio = (message) => {
    const task = message?.image_task || message?.video_task || {};
    const params = parseCanvasTaskParams(task?.params || task?.request_params);
    const metadata = parseCanvasTaskParams(task?.image_metadata);
    const metadataDetails = parseCanvasTaskParams(metadata?.metadata);
    const candidates = [
      message?.canvas_aspect_ratio,
      params.aspect_ratio,
      params.aspectRatio,
      task?.aspect_ratio,
      task?.aspectRatio,
      task?.resolution,
      task?.image_size,
      task?.imageSize,
      params.resolution,
      params.image_size,
      params.imageSize,
    ];
    for (const candidate of candidates) {
      const normalized = normalizeAspectRatioText(candidate);
      if (normalized) {
        return normalized;
      }
    }
    const dimensionSources = [task, params, metadata, metadataDetails];
    let outputWidth = 0;
    let outputHeight = 0;
    for (const source of dimensionSources) {
      if (!source || typeof source !== 'object') {
        continue;
      }
      outputWidth =
        outputWidth ||
        Number(
          source.output_width ||
            source.outputWidth ||
            source.image_width ||
            source.imageWidth ||
            source.width ||
            0,
        );
      outputHeight =
        outputHeight ||
        Number(
          source.output_height ||
            source.outputHeight ||
            source.image_height ||
            source.imageHeight ||
            source.height ||
            0,
        );
      if (outputWidth > 0 && outputHeight > 0) {
        return buildAspectRatioTextFromDimensions(outputWidth, outputHeight);
      }
      const normalizedSize = normalizeAspectRatioText(
        source.output_size_text ||
          source.outputSizeText ||
          source.size_text ||
          source.sizeText ||
          source.output_size ||
          source.outputSize ||
          source.size ||
          source.dimensions,
      );
      if (normalizedSize) {
        return normalizedSize;
      }
    }
    return '';
  };

  const getCanvasMessageAspectRatio = (message) => {
    return getCanvasMessageExplicitAspectRatio(message) || '1 / 1';
  };

  const getCanvasMediaCardSize = (
    aspectRatio,
    { batchLayout = false, batchAspectRatio = '' } = {},
  ) => {
    if (batchLayout) {
      return {
        width: '100%',
        maxWidth: '100%',
        maxHeight: 'none',
        aspectRatio: batchAspectRatio || aspectRatio || '1 / 1',
      };
    }
    const match = String(aspectRatio || '').match(
      /^(\d+(?:\.\d+)?)\s*\/\s*(\d+(?:\.\d+)?)$/,
    );
    const ratio = match ? Number(match[1]) / Number(match[2]) : 1;
    const safeRatio = Number.isFinite(ratio) && ratio > 0 ? ratio : 1;
    const maxWidth = isMobile ? 320 : 420;
    const maxHeight = isMobile ? 280 : 360;
    const width = Math.min(maxWidth, Math.round(maxHeight * safeRatio));
    return {
      width: `${width}px`,
      maxWidth: '100%',
      aspectRatio,
    };
  };

  const mergeCanvasMessageTaskDetail = (message, detail) => {
    if (!detail) {
      return message;
    }
    if (getCanvasMessageTaskType(message) === 'video_generation') {
      return {
        ...message,
        video_task: {
          ...(message.video_task || {}),
          ...detail,
        },
      };
    }
    return {
      ...message,
      image_task: {
        ...(message.image_task || {}),
        ...detail,
      },
    };
  };

  const loadCanvasMessageTaskDetail = async (message) => {
    const task = message?.image_task || message?.video_task || null;
    const taskId = task?.id;
    if (!taskId) {
      return task;
    }

    const taskType = getCanvasMessageTaskType(message);
    const cacheKey = `${taskType || 'task'}:${taskId}`;
    if (canvasMessageDetailCacheRef.current.has(cacheKey)) {
      return canvasMessageDetailCacheRef.current.get(cacheKey);
    }

    const requestSeq =
      (canvasMessageDetailRequestSeqRef.current.get(cacheKey) || 0) + 1;
    canvasMessageDetailRequestSeqRef.current.set(cacheKey, requestSeq);

    try {
      const endpoint =
        taskType === 'video_generation'
          ? `/api/video-generation/tasks/${taskId}`
          : `/api/image-generation/tasks/${taskId}`;
      const res = await API.get(endpoint);
      if (
        requestSeq !== canvasMessageDetailRequestSeqRef.current.get(cacheKey)
      ) {
        return task;
      }
      if (res.data.success && res.data.data) {
        canvasMessageDetailCacheRef.current.set(cacheKey, res.data.data);
        return res.data.data;
      }
    } catch (error) {
      console.error('Failed to load canvas message task detail:', error);
    }

    canvasMessageDetailCacheRef.current.set(cacheKey, task);
    return task;
  };

  const attachPendingReferencesToMessages = (messages, referenceFiles) => {
    if (!Array.isArray(messages) || messages.length === 0) {
      return messages || [];
    }
    const files = (referenceFiles || []).filter((file) => file?.url);
    if (files.length === 0) {
      return messages;
    }

    return messages.map((message) => {
      if (!message?.task_id || message.role === 'user') {
        return message;
      }
      return {
        ...message,
        reference_images: files.map((file) => ({
          uid: file.uid || file.name || file.url,
          name: file.name || t('参考图'),
          url: file.url,
        })),
      };
    });
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

  const removeImageTasksFromLocalState = (taskIds, options = {}) => {
    const idSet = buildComparableIdSet(taskIds);
    if (idSet.size === 0) {
      return;
    }
    const hasExplicitDecrementTotal = Object.prototype.hasOwnProperty.call(
      options,
      'decrementTotal',
    );
    const parsedDecrementTotal = Number(options.decrementTotal);
    const fallbackDecrementTotal = tasks.filter((task) =>
      idSet.has(normalizeComparableId(task.id)),
    ).length;
    const decrementTotal =
      hasExplicitDecrementTotal && Number.isFinite(parsedDecrementTotal)
        ? parsedDecrementTotal
        : fallbackDecrementTotal;
    const selectedTaskRemoved =
      selectedTask && idSet.has(normalizeComparableId(selectedTask.id));

    setTasks((prevTasks) =>
      prevTasks.filter((task) => !idSet.has(normalizeComparableId(task.id))),
    );
    setSelectedTaskIds((prev) => {
      const next = new Set(prev);
      idSet.forEach((taskId) => {
        next.delete(taskId);
        next.delete(Number(taskId));
      });
      return next;
    });
    setTaskTotal((prev) => Math.max(0, prev - decrementTotal));
    if (selectedTaskRemoved) {
      taskDetailRequestSeqRef.current += 1;
      setTaskModalVisible(false);
    }
    setSelectedTask((prev) => {
      if (!prev || !idSet.has(normalizeComparableId(prev.id))) {
        return prev;
      }
      return null;
    });
  };

  const removeVideoTasksFromLocalState = (taskIds, options = {}) => {
    const idSet = buildComparableIdSet(taskIds);
    if (idSet.size === 0) {
      return;
    }
    const hasExplicitDecrementTotal = Object.prototype.hasOwnProperty.call(
      options,
      'decrementTotal',
    );
    const parsedDecrementTotal = Number(options.decrementTotal);
    const fallbackDecrementTotal = videoTasks.filter((task) =>
      idSet.has(normalizeComparableId(task.id)),
    ).length;
    const decrementTotal =
      hasExplicitDecrementTotal && Number.isFinite(parsedDecrementTotal)
        ? parsedDecrementTotal
        : fallbackDecrementTotal;
    const selectedTaskRemoved =
      videoSelectedTask && idSet.has(normalizeComparableId(videoSelectedTask.id));

    setVideoTasks((prevTasks) =>
      prevTasks.filter((task) => !idSet.has(normalizeComparableId(task.id))),
    );
    setVideoSelectedTaskIds((prev) => {
      const next = new Set(prev);
      idSet.forEach((taskId) => {
        next.delete(taskId);
        next.delete(Number(taskId));
      });
      return next;
    });
    setVideoTaskTotal((prev) => Math.max(0, prev - decrementTotal));
    if (selectedTaskRemoved) {
      setVideoTaskModalVisible(false);
    }
    setVideoSelectedTask((prev) => {
      if (!prev || !idSet.has(normalizeComparableId(prev.id))) {
        return prev;
      }
      return null;
    });
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
      const successfulResultIndexes = results
        .map((result, index) =>
          result.status === 'fulfilled' && result.value.data?.success
            ? index
            : -1,
        )
        .filter((index) => index >= 0);
      const successCount = successfulResultIndexes.length;
      const failCount = results.length - successCount;

      if (successCount > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successCount }));
        const selectedIds = Array.from(selectedTaskIds);
        const deletedTaskIds = new Set(
          successfulResultIndexes.map((index) => selectedIds[index]),
        );
        removeImageTasksFromLocalState(deletedTaskIds);
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
      const selectedIds = Array.from(videoSelectedTaskIds);
      const successIds = new Set(
        selectedIds.filter((taskId, index) => {
          const result = results[index];
          return result?.status === 'fulfilled' && result.value.data?.success;
        }),
      );
      if (successIds.size > 0) {
        showSuccess(t('成功删除 {{count}} 个任务', { count: successIds.size }));
        removeVideoTasksFromLocalState(successIds);
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
      };

      sseRef.current = eventSource;
    } catch (error) {
      console.error('Failed to connect SSE:', error);
      setSseConnected(false);
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
    updateCanvasMessageTask(updatedTask, CANVAS_MODE_IMAGE);
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
    updateCanvasMessageTask(updatedTask, CANVAS_MODE_VIDEO);
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
      let canvasReferenceFiles = [];

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
        canvasReferenceFiles = base64Images
          .map((url, index) => ({
            uid: referenceImages[index]?.uid || `reference-${index}`,
            name: referenceImages[index]?.name || `${t('参考图')}-${index + 1}`,
            url,
          }))
          .filter((file) => file.url);
      }
      if (modelSupportsMaskEditing(selectedModelData) && maskImage?.fileInstance) {
        params.mask = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = (e) => resolve(e.target.result);
          reader.onerror = reject;
          reader.readAsDataURL(maskImage.fileInstance);
        });
      }

      const canvasSession = await ensureCanvasSession(CANVAS_MODE_IMAGE);
      const clientRequestId = generateCanvasClientRequestId();
      // UI uses inspiration wording; backend task DTO still expects prompt.
      const taskPayload = {
        model_id: selectedModel,
        group: selectedGroup,
        prompt: inspiration.trim(),
        request_endpoint: selectedModelData.request_endpoint,
        params: JSON.stringify(params),
        client_request_id: clientRequestId,
      };
      const submittedAt = Math.floor(Date.now() / 1000);
      replaceCanvasMessagesForSession(canvasSession.id, clientRequestId, [
        {
          id: `${clientRequestId}-user`,
          role: 'user',
          prompt: inspiration.trim(),
          created_time: submittedAt,
          client_request_id: clientRequestId,
          canvas_aspect_ratio: aspectRatio || '',
        },
        ...Array.from({ length: taskCount }, (_, index) => ({
          id:
            index === 0
              ? `${clientRequestId}-assistant`
              : `${clientRequestId}-assistant-${index + 1}`,
          role: 'assistant',
          prompt: inspiration.trim(),
          status: 'generating',
          task_type: 'image_generation',
          created_time: submittedAt,
          client_request_id: clientRequestId,
          canvas_aspect_ratio: aspectRatio || '',
          image_task: {
            id:
              index === 0
                ? `pending-${clientRequestId}`
                : `pending-${clientRequestId}-${index + 1}`,
            status: 'generating',
            prompt: inspiration.trim(),
            model_id: selectedModel,
            selected_group: selectedGroup,
            aspect_ratio: aspectRatio || '',
            thumbnail_url: '',
            image_url: '',
            error_message: '',
          },
        })),
      ], {
        canvasAspectRatio: aspectRatio || '',
      });
      const results = await Promise.allSettled(
        Array.from({ length: taskCount }, () =>
          API.post(
            `/api/canvas/sessions/${canvasSession.id}/messages`,
            taskPayload,
          ),
        ),
      );

      const createdTasks = [];
      const createdMessages = [];
      let firstError = '';

      results.forEach((result) => {
        if (result.status === 'fulfilled' && result.value.data.success) {
          const messages = result.value.data.data || [];
          createdMessages.push(...messages);
          messages.forEach((message) => {
            if (message?.image_task) {
              createdTasks.push(message.image_task);
            }
          });
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
        replaceCanvasMessagesForSession(
          canvasSession.id,
          clientRequestId,
          attachPendingReferencesToMessages(createdMessages, canvasReferenceFiles),
          { canvasAspectRatio: aspectRatio || '' },
        );
        loadCanvasSessions(CANVAS_MODE_IMAGE, { silent: true });
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
        removeCanvasMessagesForRequest(canvasSession.id, clientRequestId);
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
    let canvasSession = null;
    let clientRequestId = '';
    try {
      let base64Image = '';
      let canvasReferenceFiles = [];
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
      if (base64Image) {
        canvasReferenceFiles = [
          {
            uid: videoReferenceImage.uid || 'video-reference-0',
            name: videoReferenceImage.name || t('首帧图'),
            url: base64Image,
          },
        ];
      }
      const params = {
        duration: videoDuration,
        resolution: videoResolution,
        aspect_ratio: videoAspectRatio,
      };
      if (base64Image) {
        params.reference_images = [base64Image];
      }
      canvasSession = await ensureCanvasSession(CANVAS_MODE_VIDEO);
      clientRequestId = generateCanvasClientRequestId();
      const taskPayload = {
        model_id: videoSelectedModel,
        prompt: videoPrompt.trim(),
        request_endpoint: videoSelectedModelData.request_endpoint,
        params: JSON.stringify(params),
        client_request_id: clientRequestId,
      };
      replaceCanvasMessagesForSession(
        canvasSession.id,
        clientRequestId,
        [
          {
            id: `${clientRequestId}-user`,
            role: 'user',
            prompt: videoPrompt.trim(),
            created_time: Math.floor(Date.now() / 1000),
            client_request_id: clientRequestId,
            canvas_aspect_ratio: videoAspectRatio || '',
          },
          {
            id: `${clientRequestId}-assistant`,
            role: 'assistant',
            prompt: videoPrompt.trim(),
            status: 'queued',
            task_type: 'video_generation',
            created_time: Math.floor(Date.now() / 1000),
            client_request_id: clientRequestId,
            canvas_aspect_ratio: videoAspectRatio || '',
            reference_images: canvasReferenceFiles,
            video_task: {
              id: `pending-${clientRequestId}`,
              status: 'queued',
              prompt: videoPrompt.trim(),
              model_id: videoSelectedModel,
              aspect_ratio: videoAspectRatio || '',
              resolution: videoResolution || '',
              duration: videoDuration || 0,
              thumbnail_url: '',
              video_url: '',
              fail_reason: '',
            },
          },
        ],
        {
          canvasAspectRatio: videoAspectRatio || '',
        },
      );
      const res = await API.post(
        `/api/canvas/sessions/${canvasSession.id}/messages`,
        taskPayload,
      );
      if (!res.data.success) {
        removeCanvasMessagesForRequest(canvasSession.id, clientRequestId);
        showError(res.data.message || t('创建视频任务失败'));
        return;
      }
      const createdMessages = res.data.data || [];
      const newTask =
        createdMessages.find((message) => message?.video_task)?.video_task ||
        null;
      showSuccess(t('视频任务已创建，正在生成中...'));
      replaceCanvasMessagesForSession(
        canvasSession.id,
        clientRequestId,
        attachPendingReferencesToMessages(createdMessages, canvasReferenceFiles),
        { canvasAspectRatio: videoAspectRatio || '' },
      );
      loadCanvasSessions(CANVAS_MODE_VIDEO, { silent: true });
      setVideoSelectedTask(newTask);
      if (
        newTask &&
        videoTaskPage === 1 &&
        !videoTaskStatusFilter &&
        !videoTaskModelFilter &&
        !videoTaskTimeFilter
      ) {
        setVideoTasks((prev) => [newTask, ...prev].slice(0, videoTaskPageSize));
      } else {
        loadVideoTasks();
      }
      if (newTask) {
        setVideoTaskTotal((prev) => prev + 1);
      }
    } catch (error) {
      if (canvasSession?.id && clientRequestId) {
        removeCanvasMessagesForRequest(canvasSession.id, clientRequestId);
      }
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

  const getAssetComparableKey = (asset) =>
    normalizeComparableId(getAssetKey(asset));

  const currentCanvasAssetIds = canvasAssets
    .map((asset) => getAssetComparableKey(asset))
    .filter((assetId) => assetId !== '');
  const selectedCanvasAssets = canvasAssets.filter((asset) =>
    selectedCanvasAssetIds.has(getAssetComparableKey(asset)),
  );
  const selectedCanvasAssetCount = selectedCanvasAssets.length;
  const allCurrentCanvasAssetsSelected =
    currentCanvasAssetIds.length > 0 &&
    currentCanvasAssetIds.every((assetId) =>
      selectedCanvasAssetIds.has(assetId),
    );
  const partiallyCurrentCanvasAssetsSelected =
    selectedCanvasAssetCount > 0 && !allCurrentCanvasAssetsSelected;

  const readAssetStringValue = (sources, keys) => {
    for (const source of sources) {
      if (!source || typeof source !== 'object') {
        continue;
      }
      for (const key of keys) {
        const value = source[key];
        if (value === undefined || value === null) {
          continue;
        }
        const text = String(value).trim();
        if (text) {
          return text;
        }
      }
    }
    return '';
  };

  const readAssetPositiveNumber = (sources, keys) => {
    const value = readAssetStringValue(sources, keys);
    if (!value) {
      return 0;
    }
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed) : 0;
  };

  const parseAssetDimensionText = (value) => {
    const match = String(value || '')
      .trim()
      .match(/^(\d+)\s*[xX*×]\s*(\d+)$/);
    if (!match) {
      return '';
    }
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!Number.isFinite(width) || !Number.isFinite(height)) {
      return '';
    }
    return width > 0 && height > 0 ? `${width}x${height}` : '';
  };

  const joinAssetMetaParts = (parts) => {
    const seen = new Set();
    return parts
      .map((part) => String(part || '').trim())
      .filter((part) => {
        if (!part || seen.has(part)) {
          return false;
        }
        seen.add(part);
        return true;
      })
      .join(' · ');
  };

  const getAssetMetadataSources = (asset) => {
    const params = parseCanvasTaskParams(asset?.params);
    const metadata = parseCanvasTaskParams(asset?.image_metadata);
    const metadataDetails = parseCanvasTaskParams(metadata.metadata);
    return { params, metadata, metadataDetails };
  };

  const getAssetDisplayName = (asset) =>
    asset?.display_name || asset?.model_name || '-';

  const getAssetTaskIdText = (asset) => {
    const taskId = asset?.task_id;
    const id = asset?.id;
    if (taskId && id && taskId !== id) {
      return `${taskId} / ${id}`;
    }
    return taskId || id || '-';
  };

  const getAssetSizeText = (asset) => {
    const { params, metadata, metadataDetails } = getAssetMetadataSources(asset);
    const metadataSources = [metadata, metadataDetails];
    const allSources = [params, metadata, metadataDetails];
    const width = readAssetPositiveNumber(metadataSources, [
      'width',
      'output_width',
      'image_width',
      'outputWidth',
      'imageWidth',
    ]);
    const height = readAssetPositiveNumber(metadataSources, [
      'height',
      'output_height',
      'image_height',
      'outputHeight',
      'imageHeight',
    ]);
    if (width > 0 && height > 0) {
      return `${width}x${height}`;
    }

    const dimensionText = parseAssetDimensionText(
      readAssetStringValue(metadataSources, [
        'size',
        'output_size',
        'dimensions',
        'outputSize',
      ]),
    );
    if (dimensionText) {
      return dimensionText;
    }

    return (
      joinAssetMetaParts([
        readAssetStringValue(allSources, ['aspect_ratio', 'aspectRatio']),
        readAssetStringValue([params], ['resolution', 'image_size', 'imageSize']),
        readAssetStringValue(allSources, [
          'size',
          'output_size',
          'dimensions',
          'outputSize',
        ]),
      ]) || '-'
    );
  };

  const handleCanvasAssetSelect = (asset, checked) => {
    const assetId = getAssetComparableKey(asset);
    if (!assetId) {
      return;
    }
    setSelectedCanvasAssetIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(assetId);
      } else {
        next.delete(assetId);
      }
      return next;
    });
  };

  const handleCanvasAssetSelectAll = (checked) => {
    setSelectedCanvasAssetIds(checked ? new Set(currentCanvasAssetIds) : new Set());
  };

  const removeDeletedCanvasAssetsFromState = (deletedAssets) => {
    const deletedList = [].concat(deletedAssets || []);
    const deletedIds = deletedList
      .map((asset) => getAssetComparableKey(asset))
      .filter((assetId) => assetId !== '');
    if (deletedIds.length === 0) {
      return;
    }
    const deletedIdSet = new Set(deletedIds);

    setCanvasAssets((prev) =>
      prev.filter((item) => !deletedIdSet.has(getAssetComparableKey(item))),
    );
    setCanvasAssetsTotal((prev) => Math.max(0, prev - deletedIdSet.size));
    setSelectedCanvasAssetIds((prev) => {
      const next = new Set(prev);
      deletedIdSet.forEach((assetId) => next.delete(assetId));
      return next;
    });
    setSelectedAssetPreview((prev) => {
      if (!prev || !deletedIdSet.has(getAssetComparableKey(prev))) {
        return prev;
      }
      return null;
    });
    removeImageTasksFromLocalState(deletedIds, {
      decrementTotal: deletedList.filter((asset) =>
        imageTaskMatchesCurrentTaskFilters({
          ...asset,
          id: getAssetKey(asset),
          status: 'success',
        }),
      ).length,
    });
  };

  const handleModeChange = (mode) => {
    if (!CANVAS_MODES.includes(mode)) {
      return;
    }
    setGenerationMode(mode);
    setMobileTaskbarVisible(false);
  };

  const selectCanvasSession = (session) => {
    const mode = CANVAS_MODES.includes(session?.mode)
      ? session.mode
      : CANVAS_MODE_IMAGE;
    if (!session?.id) {
      return;
    }
    blankCanvasSelectionModesRef.current[mode] = false;
    setGenerationMode(mode);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [mode]: session.id,
    }));
    setMobileTaskbarVisible(false);
  };

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_IMAGE ||
      selectedCanvasSession?.mode !== CANVAS_MODE_IMAGE ||
      !selectedCanvasSession.current_model ||
      models.length === 0
    ) {
      return;
    }
    const model = models.find(
      (item) => item.request_model === selectedCanvasSession.current_model,
    );
    if (!model) {
      return;
    }
    if (model.model_series) {
      setSelectedSeries(model.model_series);
    }
    setSelectedModel(model.request_model);
  }, [
    generationMode,
    models,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
  ]);

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_VIDEO ||
      selectedCanvasSession?.mode !== CANVAS_MODE_VIDEO ||
      !selectedCanvasSession.current_model ||
      videoModels.length === 0
    ) {
      return;
    }
    const model = videoModels.find(
      (item) => item.request_model === selectedCanvasSession.current_model,
    );
    if (!model) {
      return;
    }
    if (model.model_series) {
      setVideoSelectedSeries(model.model_series);
    }
    setVideoSelectedModel(model.request_model);
  }, [
    generationMode,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
    videoModels,
  ]);

  useEffect(() => {
    if (
      generationMode !== CANVAS_MODE_CHAT ||
      selectedCanvasSession?.mode !== CANVAS_MODE_CHAT ||
      !selectedCanvasSession.current_model
    ) {
      return;
    }
    if (
      chatModels.length > 0 &&
      !chatModels.includes(selectedCanvasSession.current_model)
    ) {
      return;
    }
    setChatModel(selectedCanvasSession.current_model);
  }, [
    chatModels,
    generationMode,
    selectedCanvasSession?.id,
    selectedCanvasSession?.current_model,
    selectedCanvasSession?.mode,
  ]);

  const handleNewBlankChat = () => {
    blankCanvasSelectionModesRef.current[CANVAS_MODE_CHAT] = true;
    setGenerationMode(CANVAS_MODE_CHAT);
    setSelectedCanvasSessionIds((prev) => ({
      ...prev,
      [CANVAS_MODE_CHAT]: null,
    }));
    setChatPrompt('');
    setChatAttachments([]);
    setMobileTaskbarVisible(false);
  };

  const handleProjectEntryClick = () => {
    showError(t('暂未开放'));
  };

  const handleSendChatMessage = async () => {
    const prompt = chatPrompt.trim();
    if (!prompt && chatAttachments.length === 0) {
      showError(t('请输入消息'));
      return;
    }
    try {
      const canvasSession = await ensureCanvasSession(CANVAS_MODE_CHAT);
      const res = await API.post(
        `/api/canvas/sessions/${canvasSession.id}/messages`,
        { prompt: prompt || t('已添加素材引用'), model_id: chatModel },
      );
      if (!res.data.success) {
        showError(res.data.message || t('发送失败'));
        return;
      }
      appendCanvasMessagesForSession(canvasSession.id, res.data.data || []);
      loadCanvasSessions(CANVAS_MODE_CHAT, { silent: true });
      setChatPrompt('');
      setChatAttachments([]);
    } catch (error) {
      showError(error.message || t('发送失败'));
    }
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
      removeDeletedCanvasAssetsFromState([asset]);
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  };

  const deleteSelectedCanvasAssets = async () => {
    if (selectedCanvasAssets.length === 0) {
      showError(t('请先选择要删除的资产'));
      return;
    }
    setAssetBatchDeleting(true);
    try {
      const results = await Promise.allSettled(
        selectedCanvasAssets.map((asset) =>
          API.delete(`/api/image-generation/tasks/${getAssetKey(asset)}`),
        ),
      );
      const successfulAssets = [];
      results.forEach((result, index) => {
        if (result.status === 'fulfilled' && result.value?.data?.success) {
          successfulAssets.push(selectedCanvasAssets[index]);
        }
      });
      const successCount = successfulAssets.length;
      const failCount = results.length - successCount;

      if (successCount > 0) {
        removeDeletedCanvasAssetsFromState(successfulAssets);
        showSuccess(t('删除成功 {{count}} 个任务', { count: successCount }));
      }
      if (failCount > 0) {
        showError(t('删除失败 {{count}} 个任务', { count: failCount }));
      }
    } catch (error) {
      showError(error.message || t('批量删除失败'));
    } finally {
      setAssetBatchDeleting(false);
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
      background: 'var(--semi-color-fill-0)',
    },
    leftPanel: {
      width: isMobile ? '100%' : 300,
      minWidth: isMobile ? 0 : 300,
      display: 'flex',
      flexDirection: 'column',
      borderRight: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      overflow: 'hidden',
    },
    sidebarHeader: {
      height: 40,
      flexShrink: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'flex-end',
      padding: '8px 10px 0',
    },
    sidebarIconButton: {
      width: 30,
      height: 30,
      minWidth: 30,
      borderRadius: 8,
    },
    sidebarNav: {
      display: 'flex',
      flexDirection: 'column',
      gap: 6,
      padding: isMobile ? 10 : '6px 10px 10px',
    },
    sidebarNavItem: {
      width: '100%',
      minHeight: 42,
      border: '1px solid transparent',
      borderRadius: 8,
      background: 'transparent',
      color: 'var(--semi-color-text-0)',
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      padding: '0 12px',
      cursor: 'pointer',
      textAlign: 'left',
      fontSize: 14,
      transition: 'background 0.16s, border-color 0.16s, color 0.16s',
    },
    sidebarNavItemActive: {
      borderColor: 'var(--semi-color-primary-light-default)',
      background: 'var(--semi-color-primary-light-default)',
      color: 'var(--semi-color-primary)',
    },
    sidebarNavItemMuted: {
      color: 'var(--semi-color-text-2)',
    },
    sidebarNavIcon: {
      width: 18,
      minWidth: 18,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    sidebarNavLabel: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    taskList: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      display: 'flex',
      flexDirection: 'column',
      gap: 6,
      padding: '0 10px 10px',
    },
    taskListItem: {
      width: '100%',
      minHeight: 44,
      borderRadius: 8,
      border: '1px solid transparent',
      background: 'transparent',
      padding: '8px 10px',
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      textAlign: 'left',
      cursor: 'pointer',
      transition: 'border-color 0.16s, background 0.16s, color 0.16s',
    },
    taskListItemActive: {
      borderColor: 'var(--semi-color-primary-light-default)',
      background: 'var(--semi-color-primary-light-default)',
    },
    taskListText: {
      minWidth: 0,
      flex: 1,
    },
    taskListTitle: {
      fontSize: 14,
      fontWeight: 600,
      color: 'var(--semi-color-text-0)',
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    sessionListItem: {
      paddingRight: 8,
    },
    sessionTypeIcon: {
      width: 22,
      minWidth: 22,
      height: 22,
      borderRadius: 6,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--semi-color-text-2)',
      background: 'var(--semi-color-fill-0)',
    },
    sessionListTitle: {
      fontSize: 14,
      fontWeight: 600,
      color: 'var(--semi-color-text-0)',
      lineHeight: 1.35,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    sessionMenuButton: {
      width: 28,
      height: 28,
      minWidth: 28,
      borderRadius: 8,
      flexShrink: 0,
    },
    rightPanel: {
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--semi-color-bg-0)',
      overflow: 'hidden',
      position: 'relative',
    },
    sidebarRevealButtonWrap: {
      position: 'absolute',
      top: 10,
      left: 10,
      zIndex: 5,
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
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      padding: isMobile ? '10px' : '12px',
    },
    promptInputRow: {
      display: 'flex',
      alignItems: 'flex-start',
      gap: 8,
      flexWrap: isMobile ? 'wrap' : 'nowrap',
      minWidth: 0,
    },
    promptInlineAssets: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      flex: '0 0 auto',
      maxWidth: isMobile ? '100%' : '48%',
      minHeight: 36,
    },
    promptInput: {
      flex: '1 1 220px',
      minWidth: isMobile ? 'min(220px, 100%)' : 0,
      border: 'none',
      background: 'transparent',
      resize: 'none',
      boxShadow: 'none',
      color: 'var(--semi-color-text-0)',
      caretColor: 'var(--semi-color-primary)',
      fontSize: 15,
      lineHeight: 1.55,
    },
    promptControls: {
      display: 'flex',
      justifyContent: 'space-between',
      gap: 8,
      alignItems: 'flex-end',
      marginTop: 8,
      flexWrap: 'wrap',
    },
    promptControlsLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      flexWrap: 'wrap',
      minWidth: 0,
      flex: 1,
    },
    promptControlsRight: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'flex-end',
      minWidth: 36,
      marginLeft: 'auto',
    },
    promptAssetBar: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      flexWrap: 'wrap',
      minHeight: 36,
    },
    uploadIconBtn: {
      width: 32,
      height: 32,
      minWidth: 32,
      borderRadius: 8,
      border: '1px dashed var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--semi-color-text-2)',
      transition: 'opacity 0.2s, border-color 0.2s, background 0.2s',
    },
    pillButton: {
      minHeight: 32,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      color: 'var(--semi-color-text-1)',
      padding: '0 10px',
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      fontSize: 13,
      lineHeight: 1,
    },
    pillButtonIconOnly: {
      width: 32,
      minWidth: 32,
      padding: 0,
      justifyContent: 'center',
      gap: 0,
    },
    pillButtonActive: {
      borderColor: 'var(--semi-color-primary-light-active)',
      background: 'var(--semi-color-primary-light-default)',
      color: 'var(--semi-color-primary)',
    },
    pillButtonMuted: {
      color: 'var(--semi-color-text-2)',
    },
    pillButtonIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      flex: '0 0 auto',
      color: 'currentColor',
    },
    pillButtonLabel: {
      maxWidth: isMobile ? 126 : 170,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    darkMenu: {
      minWidth: 240,
      maxWidth: 340,
      border: '1px solid var(--semi-color-border)',
      borderRadius: 10,
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 16px 40px rgba(15, 23, 42, 0.16)',
      padding: 6,
    },
    darkMenuItem: {
      color: 'var(--semi-color-text-0)',
      borderRadius: 8,
      margin: 0,
      padding: '8px 10px',
      whiteSpace: 'nowrap',
    },
    darkMenuItemContent: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
    },
    darkMenuItemActive: {
      background: 'var(--semi-color-primary-light-default)',
      color: 'var(--semi-color-primary)',
    },
    imageParamPanel: {
      width: isMobile ? 'calc(100vw - 32px)' : 380,
      maxWidth: 'calc(100vw - 32px)',
      border: '1px solid var(--semi-color-border)',
      borderRadius: 10,
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 18px 48px rgba(15, 23, 42, 0.18)',
      padding: isMobile ? 12 : 14,
    },
    imageParamSection: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
    },
    imageParamSectionTitle: {
      fontSize: 13,
      fontWeight: 650,
      color: 'var(--semi-color-text-0)',
      lineHeight: 1.3,
    },
    imageParamOptionRow: {
      display: 'flex',
      gap: 8,
      flexWrap: 'wrap',
      minWidth: 0,
    },
    imageParamOption: {
      minHeight: 32,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      color: 'var(--semi-color-text-1)',
      padding: '0 11px',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: 13,
      lineHeight: 1.2,
      cursor: 'pointer',
      maxWidth: '100%',
      whiteSpace: 'nowrap',
    },
    imageParamOptionActive: {
      borderColor: 'var(--semi-color-primary-light-active)',
      background: 'var(--semi-color-primary-light-default)',
      color: 'var(--semi-color-primary)',
      fontWeight: 650,
    },
    imageParamDivider: {
      height: 1,
      background: 'var(--semi-color-border)',
      margin: '12px 0',
    },
    imageParamSizeRow: {
      display: 'grid',
      gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
      gap: 8,
    },
    imageParamSizeField: {
      minHeight: 34,
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
      color: 'var(--semi-color-text-0)',
      padding: '0 10px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 8,
      minWidth: 0,
    },
    imageParamSizeLabel: {
      flex: '0 0 auto',
      color: 'var(--semi-color-text-2)',
      fontSize: 12,
    },
    imageParamSizeValue: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      fontSize: 13,
      fontWeight: 650,
    },
    modelMenuOption: {
      display: 'flex',
      alignItems: 'flex-start',
      gap: 8,
      minWidth: 0,
    },
    modelMenuIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      flex: '0 0 auto',
      marginTop: 1,
      color: 'currentColor',
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
      color: 'var(--semi-color-text-2)',
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
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      cursor: 'pointer',
      color: 'var(--semi-color-text-2)',
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
    composerParameterRow: {
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      flexWrap: 'wrap',
      minWidth: 0,
    },
    filterLabel: {
      fontSize: 13,
      color: 'var(--semi-color-text-2)',
    },
    tasksGrid: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
      gap: isMobile ? 10 : 12,
      padding: 0,
      width: '100%',
      alignContent: 'start',
      flexShrink: 0,
    },
    workspaceTopbar: {
      height: isMobile ? 44 : 0,
      flexShrink: 0,
      borderBottom: 'none',
      background: 'var(--semi-color-bg-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      padding: isMobile ? '0 10px' : 0,
    },
    workspaceTopbarLeft: {
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      minWidth: 0,
    },
    workspaceBody: {
      flex: 1,
      minHeight: 0,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      background: 'var(--semi-color-bg-0)',
    },
    mainViewport: {
      flex: 1,
      minHeight: 0,
      overflowY: 'auto',
      padding: isMobile ? '10px 10px 6px' : '16px 18px 8px',
      background: 'var(--semi-color-bg-0)',
    },
    composerDock: {
      flexShrink: 0,
      borderTop: 'none',
      background: 'var(--semi-color-bg-0)',
      padding: isMobile ? '6px 10px 10px' : '8px 18px 14px',
    },
    composerShell: {
      width: '100%',
      maxWidth: 1080,
      margin: '0 auto',
      border: 'none',
      borderRadius: 0,
      background: 'transparent',
      padding: 0,
    },
    chatStream: {
      minHeight: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 10 : 12,
      maxWidth: 1080,
      margin: '0 auto',
      padding: isMobile ? '8px 0 12px' : '12px 0 18px',
      width: '100%',
    },
    canvasStreamEmpty: {
      maxWidth: 1080,
      margin: '0 auto',
      width: '100%',
      padding: isMobile ? '8px 0 12px' : '12px 0 18px',
    },
    canvasMessageList: {
      display: 'flex',
      flexDirection: 'column',
      gap: isMobile ? 10 : 12,
      width: '100%',
    },
    canvasMessageRow: {
      width: 'fit-content',
      maxWidth: isMobile ? '92%' : '72%',
      borderRadius: 8,
      padding: '10px 12px',
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      color: 'var(--semi-color-text-0)',
      lineHeight: 1.55,
      wordBreak: 'break-word',
      boxShadow: '0 1px 2px rgba(15, 23, 42, 0.04)',
    },
    canvasMessageRowUser: {
      alignSelf: 'flex-end',
      background: 'var(--semi-color-bg-0)',
      borderColor: 'var(--semi-color-border)',
    },
    canvasMessageRowAssistant: {
      alignSelf: 'flex-start',
      background: 'var(--semi-color-bg-0)',
    },
    canvasGenerationRow: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      alignItems: 'flex-start',
    },
    canvasGenerationRowUser: {
      alignItems: 'flex-end',
    },
    canvasMessageRowSelected: {
      borderColor: 'var(--semi-color-border)',
      background: 'var(--semi-color-primary-light-default)',
      boxShadow: '0 8px 24px rgba(15, 23, 42, 0.08)',
    },
    canvasGenerationRowSelected: {
      filter: 'drop-shadow(0 8px 18px rgba(15, 23, 42, 0.1))',
    },
    canvasMessageHead: {
      display: 'flex',
      justifyContent: 'flex-start',
      gap: 12,
      marginBottom: 6,
    },
    canvasMessageBody: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      width: '100%',
    },
    canvasMessagePrompt: {
      maxWidth: isMobile ? '92%' : 640,
      whiteSpace: 'pre-wrap',
      color: 'var(--semi-color-text-0)',
    },
    canvasMessageRefs: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: 8,
    },
    canvasReferenceThumbWrap: {
      width: 140,
    },
    canvasReferenceThumb: {
      display: 'block',
      width: '100%',
      height: 96,
      objectFit: 'cover',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-fill-0)',
    },
    canvasGenerationCardWrap: {
      display: 'flex',
      flexDirection: 'column',
      gap: 10,
      alignItems: 'flex-start',
      width: '100%',
    },
    canvasGenerationBatchRow: {
      width: '100%',
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      alignItems: 'stretch',
    },
    canvasGenerationBatchGrid: {
      width: '100%',
      maxWidth: isMobile ? '100%' : 980,
      display: 'grid',
      gridTemplateColumns:
        isMobile
          ? 'repeat(auto-fit, minmax(min(320px, 100%), 1fr))'
          : 'repeat(auto-fit, minmax(220px, 1fr))',
      gap: isMobile ? 8 : 6,
      alignItems: 'start',
      alignSelf: 'flex-start',
    },
    canvasGenerationBatchCard: {
      minWidth: 0,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'stretch',
    },
    canvasMediaCard: {
      width: '100%',
      minWidth: 0,
      maxWidth: isMobile ? 320 : 420,
      maxHeight: isMobile ? 280 : 360,
      position: 'relative',
      overflow: 'hidden',
      borderRadius: 8,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 1px 2px rgba(15, 23, 42, 0.04)',
    },
    canvasMediaCardClickable: {
      cursor: 'zoom-in',
    },
    canvasMediaFrame: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    canvasMediaContent: {
      width: '100%',
      height: '100%',
      objectFit: 'contain',
      display: 'block',
      background: 'var(--semi-color-fill-0)',
    },
    canvasImagePreviewPanel: {
      maxWidth: isMobile ? 'calc(100vw - 24px)' : 'calc(100vw - 48px)',
      maxHeight: 'calc(100vh - 48px)',
      overflow: 'hidden',
      borderRadius: 10,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 24px 72px rgba(15, 23, 42, 0.26)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      position: 'relative',
    },
    canvasImagePreviewImage: {
      display: 'block',
      maxWidth: '100%',
      maxHeight: 'calc(100vh - 48px)',
      objectFit: 'contain',
      background: 'var(--semi-color-fill-0)',
    },
    canvasMediaStatusBody: {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      color: 'var(--semi-color-text-2)',
      padding: 16,
    },
    canvasErrorText: {
      maxWidth: '100%',
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-word',
      overflowWrap: 'anywhere',
      textAlign: 'center',
      lineHeight: 1.5,
    },
    canvasMessageResult: {
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      alignItems: 'flex-start',
    },
    canvasMessageResultFrame: {
      maxWidth: '100%',
    },
    messageResultImage: {
      display: 'block',
      width: '100%',
      maxWidth: isMobile ? '100%' : 520,
      maxHeight: isMobile ? 420 : 620,
      objectFit: 'contain',
      borderRadius: 8,
      background: 'var(--semi-color-fill-0)',
      cursor: 'pointer',
    },
    messageResultVideo: {
      display: 'block',
      width: '100%',
      maxWidth: isMobile ? '100%' : 560,
      maxHeight: isMobile ? 420 : 620,
      borderRadius: 8,
      background: '#000',
    },
    messagePending: {
      minWidth: 180,
      minHeight: 96,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      borderRadius: 8,
      background: 'var(--semi-color-fill-0)',
      color: 'var(--semi-color-text-2)',
    },
    assetGrid: {
      display: 'grid',
      gridTemplateColumns: isMobile
        ? 'repeat(2, minmax(0, 1fr))'
        : 'repeat(3, minmax(0, 1fr))',
      gap: 12,
    },
    assetCard: {
      position: 'relative',
      border: '1px solid var(--semi-color-border)',
      borderRadius: 10,
      overflow: 'hidden',
      background: 'var(--semi-color-bg-0)',
      display: 'flex',
      flexDirection: 'column',
      cursor: 'pointer',
      minWidth: 0,
      transition: 'border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease',
    },
    assetSelect: {
      position: 'absolute',
      top: 6,
      left: 6,
      zIndex: 1,
      padding: 3,
      borderRadius: 6,
      background: 'rgba(0, 0, 0, 0.48)',
      lineHeight: 1,
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
      borderTop: '1px solid var(--semi-color-border)',
    },
    assetCardMeta: {
      padding: '8px',
      display: 'flex',
      flexDirection: 'column',
      gap: 2,
      minWidth: 0,
    },
    assetPreviewOverlay: {
      position: 'fixed',
      inset: 0,
      zIndex: 1200,
      padding: isMobile ? 12 : 24,
      background: 'rgba(15, 23, 42, 0.52)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    assetPreviewPanel: {
      width: 'min(100%, 980px)',
      maxHeight: 'calc(100vh - 48px)',
      display: 'grid',
      gridTemplateColumns: isMobile ? '1fr' : 'minmax(0, 1.35fr) minmax(280px, 0.65fr)',
      gap: 0,
      overflow: 'hidden',
      borderRadius: 10,
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
      boxShadow: '0 24px 72px rgba(15, 23, 42, 0.26)',
    },
    assetPreviewImagePane: {
      minHeight: isMobile ? 220 : 520,
      maxHeight: isMobile ? '48vh' : 'calc(100vh - 48px)',
      background: 'var(--semi-color-fill-0)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      overflow: 'hidden',
    },
    assetPreviewImage: {
      width: '100%',
      height: '100%',
      maxHeight: isMobile ? '48vh' : 'calc(100vh - 48px)',
      objectFit: 'contain',
      display: 'block',
    },
    assetPreviewInfo: {
      padding: isMobile ? 12 : 16,
      display: 'flex',
      flexDirection: 'column',
      gap: 8,
      overflowY: 'auto',
      maxHeight: isMobile ? '44vh' : 'calc(100vh - 48px)',
      minWidth: 0,
    },
    assetPreviewMetaItem: {
      display: 'flex',
      flexDirection: 'column',
      gap: 3,
      padding: 0,
      borderRadius: 0,
      background: 'transparent',
      minWidth: 0,
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
    icon = null,
    value,
    displayValue,
    options,
    onChange,
    disabled = false,
    emptyText = t('暂无可用选项'),
    extraContent = null,
  }) => {
    const dropdownKey = key;
    const iconOnly = isMobile && !!icon;
    const buttonText = label
      ? `${label} ${displayValue || t('请选择')}`
      : displayValue || t('请选择');
    const closeDropdown = () => {
      setActiveDropdownKey((current) =>
        current === dropdownKey ? '' : current,
      );
    };
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
                onClick={() => {
                  onChange(option.value);
                  closeDropdown();
                }}
              >
                <span style={styles.darkMenuItemContent}>
                  {option.icon ? (
                    <span style={styles.pillButtonIcon}>{option.icon}</span>
                  ) : null}
                  <span>{option.label}</span>
                </span>
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
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={menu}
        visible={!disabled && activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          type='button'
          aria-label={buttonText}
          title={buttonText}
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...(value !== '' && value !== null && value !== undefined
              ? styles.pillButtonActive
              : styles.pillButtonMuted),
            opacity: disabled ? 0.55 : 1,
            cursor: disabled ? 'not-allowed' : 'pointer',
          }}
          disabled={disabled}
        >
          {icon ? <span style={styles.pillButtonIcon}>{icon}</span> : null}
          {!iconOnly ? (
            <span style={styles.pillButtonLabel}>{buttonText}</span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const getAspectRatioDisplay = (value) =>
    String(value || '').trim().toLowerCase() === 'auto'
      ? t('智能')
      : String(value || '');

  const getAspectRatioSummaryDisplay = (value) =>
    String(value || '').trim().toLowerCase() === 'auto'
      ? t('智能比例')
      : String(value || '');

  const getResolutionDisplay = (value) => {
    const text = String(value || '').trim();
    const normalized = text.toUpperCase();
    if (normalized === '2K') {
      return t('高清 2K');
    }
    if (normalized === '4K') {
      return t('超清 4K');
    }
    return text;
  };

  const renderImageParamOption = ({ value, label, selected, onClick }) => (
    <button
      key={value}
      type='button'
      style={{
        ...styles.imageParamOption,
        ...(selected ? styles.imageParamOptionActive : null),
      }}
      onClick={(event) => {
        event.stopPropagation();
        onClick(value);
      }}
    >
      {label}
    </button>
  );

  const renderGenerationParametersDropdown = ({
    key,
    ariaLabel,
    aspectRatios,
    resolutions,
    aspectRatioValue,
    resolutionValue,
    onAspectRatioChange,
    onResolutionChange,
    showAspectRatioSelector,
    showResolutionSelector,
    quantityValue = 0,
    quantityOptions = [],
    onQuantityChange = null,
  }) => {
    const hasAspectRatios = showAspectRatioSelector && aspectRatios.length > 0;
    const hasResolutions = showResolutionSelector && resolutions.length > 0;
    const iconOnly = isMobile;
    const hasQuantitySelector =
      Array.isArray(quantityOptions) &&
      quantityOptions.length > 0 &&
      typeof onQuantityChange === 'function';
    if (!hasAspectRatios && !hasResolutions && !hasQuantitySelector) {
      return null;
    }

    const dropdownKey = key;
    const dimensionPreview = getImageDimensionPreview(
      hasResolutions ? resolutionValue : '',
      hasAspectRatios ? aspectRatioValue : '',
    );
    const displayParts = [];
    if (hasAspectRatios && aspectRatioValue) {
      displayParts.push(getAspectRatioSummaryDisplay(aspectRatioValue));
    }
    if (hasResolutions && resolutionValue) {
      displayParts.push(getResolutionDisplay(resolutionValue));
    }
    if (hasQuantitySelector && quantityValue > 0) {
      displayParts.push(`${t('数量')} ${quantityValue}`);
    }
    const displayValue = displayParts.length > 0 ? displayParts.join(' | ') : t('请选择');
    const buttonLabel =
      displayValue && displayValue !== t('请选择')
        ? `${ariaLabel} ${displayValue}`
        : ariaLabel;
    const panel = (
      <div
        style={styles.imageParamPanel}
        onClick={(event) => event.stopPropagation()}
      >
        {hasAspectRatios ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('选择比例')}</div>
            <div style={styles.imageParamOptionRow}>
              {aspectRatios.map((ratio) =>
                renderImageParamOption({
                  value: ratio,
                  label: getAspectRatioDisplay(ratio),
                  selected: ratio === aspectRatioValue,
                  onClick: onAspectRatioChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        {hasAspectRatios && hasResolutions ? (
          <div style={styles.imageParamDivider} />
        ) : null}
        {hasResolutions ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('选择分辨率')}</div>
            <div style={styles.imageParamOptionRow}>
              {resolutions.map((item) =>
                renderImageParamOption({
                  value: item,
                  label: getResolutionDisplay(item),
                  selected: item === resolutionValue,
                  onClick: onResolutionChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        {(hasAspectRatios || hasResolutions) && hasQuantitySelector ? (
          <div style={styles.imageParamDivider} />
        ) : null}
        {hasQuantitySelector ? (
          <div style={styles.imageParamSection}>
            <div style={styles.imageParamSectionTitle}>{t('生成数量')}</div>
            <div style={styles.imageParamOptionRow}>
              {quantityOptions.map((item) =>
                renderImageParamOption({
                  value: item,
                  label: String(item),
                  selected: item === quantityValue,
                  onClick: onQuantityChange,
                }),
              )}
            </div>
          </div>
        ) : null}
        <div style={styles.imageParamDivider} />
        <div style={styles.imageParamSection}>
          <div style={styles.imageParamSectionTitle}>{t('尺寸')}</div>
          <div style={styles.imageParamSizeRow}>
            <div style={styles.imageParamSizeField}>
              <span style={styles.imageParamSizeLabel}>W</span>
              <span style={styles.imageParamSizeValue}>{dimensionPreview.width}</span>
            </div>
            <div style={styles.imageParamSizeField}>
              <span style={styles.imageParamSizeLabel}>H</span>
              <span style={styles.imageParamSizeValue}>{dimensionPreview.height}</span>
            </div>
          </div>
        </div>
      </div>
    );

    return (
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={panel}
        visible={activeDropdownKey === dropdownKey}
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          type='button'
          aria-label={buttonLabel}
          title={buttonLabel}
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
            ...styles.pillButtonActive,
            maxWidth: isMobile ? '100%' : 260,
          }}
        >
          <IconRealSizeStroked size='small' />
          {!iconOnly ? (
            <span style={styles.pillButtonLabel}>{displayValue}</span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
    );
  };

  const renderImageParametersDropdown = () =>
    renderGenerationParametersDropdown({
      key: 'image-parameters',
      ariaLabel: t('图片参数'),
      aspectRatios: availableAspectRatios,
      resolutions: availableResolutions,
      aspectRatioValue: aspectRatio,
      resolutionValue: resolution,
      onAspectRatioChange: setAspectRatio,
      onResolutionChange: setResolution,
      showAspectRatioSelector: showImageAspectRatioSelector,
      showResolutionSelector: showImageResolutionSelector,
      quantityValue: quantity,
      quantityOptions: Array.from(
        { length: DEFAULT_MAX_BATCH_TASKS },
        (_, index) => index + 1,
      ),
      onQuantityChange: (value) => setQuantity(normalizeTaskCount(value)),
    });

  const renderVideoParametersDropdown = () =>
    renderGenerationParametersDropdown({
      key: 'video-parameters',
      ariaLabel: t('视频参数'),
      aspectRatios: videoAvailableAspectRatios,
      resolutions: videoAvailableResolutions,
      aspectRatioValue: videoAspectRatio,
      resolutionValue: videoResolution,
      onAspectRatioChange: setVideoAspectRatio,
      onResolutionChange: setVideoResolution,
      showAspectRatioSelector: showVideoAspectRatioSelector,
      showResolutionSelector: showVideoResolutionSelector,
    });

  const renderModelDropdown = (isVideoMode, activeModelLabel) => {
    const dropdownKey = isVideoMode ? 'video-model' : 'image-model';
    const modelOptions = isVideoMode ? enabledVideoModels : enabledImageModels;
    const selectedValue = isVideoMode ? videoSelectedModel : selectedModel;
    const iconOnly = isMobile;
    const buttonLabel = isVideoMode
      ? `${t('选择视频模型')} ${activeModelLabel}`
      : `${t('选择图片模型')} ${activeModelLabel}`;
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
                onClick={() => {
                  handleSelect(model.request_model);
                  setActiveDropdownKey((current) =>
                    current === dropdownKey ? '' : current,
                  );
                }}
              >
                <div style={styles.modelMenuOption}>
                  <span style={styles.modelMenuIcon}>
                    {isVideoMode ? (
                      <IconVideo size='small' />
                    ) : (
                      <IconImage size='small' />
                    )}
                  </span>
                  <div style={styles.modelMenuItem}>
                    <span style={styles.modelMenuTitle}>
                      {getModelDisplayName(model)}
                    </span>
                    <span style={styles.modelMenuMeta}>
                      {model.request_model}
                    </span>
                  </div>
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
      <Dropdown
        key={dropdownKey}
        trigger='click'
        position='bottomLeft'
        render={menu}
        visible={
          modelOptions.length > 0 && activeDropdownKey === dropdownKey
        }
        onVisibleChange={(visible) => {
          setActiveDropdownKey((current) =>
            visible ? dropdownKey : current === dropdownKey ? '' : current,
          );
        }}
      >
        <button
          type='button'
          aria-label={buttonLabel}
          title={buttonLabel}
          data-canvas-model-selector={isVideoMode ? CANVAS_MODE_VIDEO : CANVAS_MODE_IMAGE}
          style={{
            ...styles.pillButton,
            ...(iconOnly ? styles.pillButtonIconOnly : null),
          }}
          disabled={modelOptions.length === 0}
        >
          {isVideoMode ? <IconVideo size='small' /> : <IconImage size='small' />}
          {!iconOnly ? (
            <span style={{ ...styles.pillButtonLabel, maxWidth: isMobile ? 140 : 260 }}>
              {`${t('模型')} ${activeModelLabel}`}
            </span>
          ) : null}
          {!iconOnly ? <IconChevronDown size='small' /> : null}
        </button>
      </Dropdown>
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

  const renderSidebarNavItem = ({
    key,
    label,
    icon,
    active = false,
    muted = false,
    onClick,
  }) => (
    <button
      key={key}
      type='button'
      style={{
        ...styles.sidebarNavItem,
        ...(active ? styles.sidebarNavItemActive : null),
        ...(muted ? styles.sidebarNavItemMuted : null),
      }}
      onClick={onClick}
    >
      <span style={styles.sidebarNavIcon}>{icon}</span>
      <span style={styles.sidebarNavLabel}>{label}</span>
    </button>
  );

  const renderCanvasSessionMenu = (session) => (
    <Dropdown.Menu style={styles.darkMenu}>
      <Dropdown.Item
        style={styles.darkMenuItem}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          renameCanvasSession(session);
        }}
      >
        {t('重命名')}
      </Dropdown.Item>
      <Dropdown.Item
        style={styles.darkMenuItem}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          toggleCanvasSessionPin(session);
        }}
      >
        {session.pinned ? t('取消置顶') : t('置顶')}
      </Dropdown.Item>
      <Dropdown.Item
        style={{ ...styles.darkMenuItem, color: '#fca5a5' }}
        onClick={(event) => {
          event?.domEvent?.stopPropagation?.();
          if (window.confirm(t('确认删除该会话？'))) {
            deleteCanvasSession(session);
          }
        }}
      >
        {t('删除')}
      </Dropdown.Item>
    </Dropdown.Menu>
  );

  const renderCanvasSessionIcon = (mode) => {
    if (mode === CANVAS_MODE_CHAT) {
      return <IconCommentStroked size='small' />;
    }
    if (mode === CANVAS_MODE_VIDEO) {
      return <IconVideo size='small' />;
    }
    return <IconImage size='small' />;
  };

  const renderCanvasSessionList = () => (
    <Spin spinning={unifiedCanvasSessionsLoading || deletingCanvasSession}>
      <div style={styles.taskList}>
        {unifiedCanvasSessionError ? (
          renderSidebarEmpty(
            t('会话加载失败'),
            unifiedCanvasSessionError,
            <Button
              size='small'
              icon={<IconRefresh />}
              onClick={() => {
                CANVAS_MODES.forEach((mode) => {
                  loadCanvasSessions(mode);
                });
              }}
            >
              {t('重试')}
            </Button>,
          )
        ) : unifiedCanvasSessions.length === 0 ? (
          <div style={{ minHeight: 28 }} />
        ) : (
          unifiedCanvasSessions.map((session) => {
            const sessionMode = CANVAS_MODES.includes(session.mode)
              ? session.mode
              : CANVAS_MODE_IMAGE;
            const active =
              generationMode === sessionMode &&
              session.id === selectedCanvasSessionIds[sessionMode];
            return (
              <div
                key={session.id}
                role='button'
                tabIndex={0}
                style={{
                  ...styles.taskListItem,
                  ...styles.sessionListItem,
                  ...(active ? styles.taskListItemActive : null),
                }}
                onClick={() => selectCanvasSession(session)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    selectCanvasSession(session);
                  }
                }}
              >
                <span style={styles.sessionTypeIcon}>
                  {renderCanvasSessionIcon(sessionMode)}
                </span>
                <div style={styles.taskListText}>
                  <div style={styles.sessionListTitle}>
                    {summarizeText(session.title, t('新会话'))}
                  </div>
                </div>
                <Dropdown
                  trigger='click'
                  position='bottomRight'
                  render={renderCanvasSessionMenu(session)}
                >
                  <Button
                    size='small'
                    type='tertiary'
                    aria-label={t('会话菜单')}
                    icon={<IconMore />}
                    style={styles.sessionMenuButton}
                    onClick={(event) => event.stopPropagation()}
                  />
                </Dropdown>
              </div>
            );
          })
        )}
      </div>
    </Spin>
  );

  const renderTaskSidebar = () => (
    <div style={styles.leftPanel} data-canvas-task-sidebar={generationMode}>
      {!isMobile ? (
        <div style={styles.sidebarHeader}>
          <Tooltip content={t('隐藏侧边栏')} position='bottom'>
            <Button
              type='tertiary'
              aria-label={t('隐藏侧边栏')}
              data-canvas-sidebar-collapse='true'
              icon={<IconChevronLeft />}
              style={styles.sidebarIconButton}
              onClick={() => setDesktopSidebarCollapsed(true)}
            />
          </Tooltip>
        </div>
      ) : null}
      <div style={styles.sidebarNav}>
        {renderSidebarNavItem({
          key: 'asset-library',
          label: t('资产库'),
          icon: <IconArchive />,
          onClick: () => {
            setAssetLibraryVisible(true);
            setMobileTaskbarVisible(false);
          },
        })}
        {renderSidebarNavItem({
          key: 'projects',
          label: t('项目'),
          icon: <IconExternalOpen />,
          muted: true,
          onClick: handleProjectEntryClick,
        })}
        {renderSidebarNavItem({
          key: 'new-chat',
          label: t('新聊天'),
          icon: <IconCommentStroked />,
          active:
            generationMode === CANVAS_MODE_CHAT && !selectedCanvasSessionId,
          onClick: handleNewBlankChat,
        })}
      </div>

      {renderCanvasSessionList()}
    </div>
  );

  const renderWeakDetails = (title, children) => (
    <details
      style={{
        borderTop: '1px solid var(--semi-color-border)',
        paddingTop: 10,
        marginTop: 2,
      }}
    >
      <summary
        style={{
          cursor: 'pointer',
          color: 'var(--semi-color-text-2)',
          fontSize: 12,
          lineHeight: 1.5,
        }}
      >
        {title}
      </summary>
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: 8,
          marginTop: 10,
        }}
      >
        {children}
      </div>
    </details>
  );

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
    return (
      <div style={styles.chatStream}>
        {renderCanvasMessageStream()}
      </div>
    );
  };

  const renderCanvasReferenceThumb = (file, fallbackLabel) => (
    <div
      key={file.uid}
      data-canvas-reference-thumb='true'
      style={styles.canvasReferenceThumbWrap}
    >
      <img src={file.url} alt='' style={styles.canvasReferenceThumb} />
      <Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
        {fallbackLabel}
      </Text>
    </div>
  );

  const openCanvasImagePreview = async (message, media) => {
    const initialSrc = media?.previewSrc || media?.src || '';
    if (initialSrc) {
      setSelectedCanvasImagePreview({ src: initialSrc });
    }

    const task = message?.image_task || null;
    if (!task?.id || task?.image_url) {
      return;
    }

    const detail = await loadCanvasMessageTaskDetail(message);
    const detailSrc = detail?.image_url || detail?.thumbnail_url || '';
    if (!detailSrc || detailSrc === initialSrc) {
      return;
    }

    setCanvasMessages((prev) =>
      prev.map((item) =>
        item.id === message.id ? mergeCanvasMessageTaskDetail(item, detail) : item,
      ),
    );
    setSelectedCanvasImagePreview({ src: detailSrc });
  };

  const getCanvasBatchAspectRatio = (batch) => {
    const userRatio = getCanvasMessageExplicitAspectRatio(batch?.userMessage);
    if (userRatio) {
      return userRatio;
    }
    for (const message of batch?.assistantMessages || []) {
      const ratio = getCanvasMessageExplicitAspectRatio(message);
      if (ratio) {
        return ratio;
      }
    }
    return '1 / 1';
  };

  const renderCanvasMediaCard = (
    message,
    { batchLayout = false, batchAspectRatio = '' } = {},
  ) => {
    const media = getCanvasMessageMedia(message);
    const taskType = getCanvasMessageTaskType(message);
    const isVideo = taskType === 'video_generation';
    const aspectRatio = getCanvasMessageAspectRatio(message);
    const task = message?.image_task || message?.video_task || null;
    const status = media?.status || task?.status || message?.status || '';
    const isDone =
      (isVideo && (status === 'completed' || status === 'success')) ||
      (!isVideo && status === 'success');
    const isFailed = status === 'failed';
    const canPreviewImage = isDone && media?.kind === 'image' && media.src;

    let content = (
      <div style={styles.canvasMediaStatusBody}>
        <Spin size='small' />
        <Text type='tertiary' size='small'>
          {isVideo ? t('正在生成视频') : t('正在生成图片')}
        </Text>
      </div>
    );

    if (isFailed) {
      content = (
        <div style={styles.canvasMediaStatusBody}>
          <IconClock size='small' />
          <Text type='danger' size='small' style={styles.canvasErrorText}>
            {media?.error || message.error_message || t('生成失败')}
          </Text>
        </div>
      );
    } else if (isDone && media?.kind === 'image' && media.src) {
      content = (
        <img
          data-canvas-message-result='image'
          src={media.src}
          alt=''
          style={styles.canvasMediaContent}
        />
      );
    } else if (isDone && media?.kind === 'video' && media.videoUrl) {
      content = (
        <video
          data-canvas-message-result='video'
          src={media.videoUrl}
          poster={media.src}
          controls
          style={styles.canvasMediaContent}
          onClick={() => setVideoSelectedTask(message.video_task || null)}
        />
      );
    }

    return (
      <div
        style={{
          ...styles.canvasMediaCard,
          ...(canPreviewImage ? styles.canvasMediaCardClickable : null),
          ...getCanvasMediaCardSize(aspectRatio, {
            batchLayout,
            batchAspectRatio,
          }),
        }}
        onClick={
          canPreviewImage
            ? (event) => {
                event.stopPropagation();
                openCanvasImagePreview(message, media);
              }
            : undefined
        }
      >
        <div style={styles.canvasMediaFrame}>{content}</div>
      </div>
    );
  };

  const renderCanvasGenerationCard = (
    message,
    { batchLayout = false, batchAspectRatio = '' } = {},
  ) => {
    return renderCanvasMediaCard(message, { batchLayout, batchAspectRatio });
  };

  const renderCanvasGenerationBatch = (batch) => {
    const userMessage = batch.userMessage || {};
    const batchAspectRatio = getCanvasBatchAspectRatio(batch);
    const referenceMap = new Map();
    [userMessage, ...(batch.assistantMessages || [])].forEach((message) => {
      getCanvasMessageReferenceFiles(message).forEach((file) => {
        const key = file.url || file.uid || file.name;
        if (key && !referenceMap.has(key)) {
          referenceMap.set(key, file);
        }
      });
    });
    const references = Array.from(referenceMap.values());
    return (
      <div
        key={batch.id}
        data-canvas-message-batch='image_generation'
        style={styles.canvasGenerationBatchRow}
      >
        <div
          data-canvas-message-row='true'
          style={{
            ...styles.canvasMessageRow,
            ...styles.canvasMessageRowUser,
          }}
        >
          <div style={styles.canvasMessageBody}>
            <div style={styles.canvasMessagePrompt}>
              {userMessage.prompt || t('已添加素材引用')}
            </div>
            {references.length > 0 ? (
              <div style={styles.canvasMessageRefs}>
                {references.map((file) =>
                  renderCanvasReferenceThumb(file, t('参考图')),
                )}
              </div>
            ) : null}
          </div>
        </div>
        <div style={styles.canvasGenerationBatchGrid}>
          {(batch.assistantMessages || []).map((message) => (
            <div
              key={message.id || message.task_id || message.image_task?.id}
              data-canvas-generation-batch-card='true'
              style={styles.canvasGenerationBatchCard}
              onClick={async () => {
                setSelectedCanvasMessageId(message.id);
                const detail = await loadCanvasMessageTaskDetail(message);
                if (!detail) {
                  return;
                }
                setCanvasMessages((prev) =>
                  prev.map((item) =>
                    item.id === message.id
                      ? mergeCanvasMessageTaskDetail(item, detail)
                      : item,
                  ),
                );
              }}
            >
              {renderCanvasGenerationCard(message, {
                batchLayout: true,
                batchAspectRatio,
              })}
            </div>
          ))}
        </div>
      </div>
    );
  };

  const renderCanvasMessageResult = (message) => {
    const media = getCanvasMessageMedia(message);
    if (!media) {
      return null;
    }
    if (media.kind === 'image') {
      if (media.status === 'success' && media.src) {
        return (
          <div style={styles.canvasMessageResultFrame}>
            <img
              data-canvas-message-result='image'
              src={media.src}
              alt=''
              style={styles.messageResultImage}
              onClick={() => setSelectedTask(message.image_task || null)}
            />
          </div>
        );
      }
      if (media.status === 'failed') {
        return (
          <Text type='danger' style={styles.canvasErrorText}>
            {media.error || t('生成失败')}
          </Text>
        );
      }
      return (
        <div style={styles.messagePending}>
          {media.status === 'generating' ? <Spin size='small' /> : <IconClock />}
          <span>{media.status === 'generating' ? t('生成中') : t('排队中')}</span>
        </div>
      );
    }
    if (media.kind === 'video') {
      if (media.status === 'completed' && media.videoUrl) {
        return (
          <div style={styles.canvasMessageResultFrame}>
            <video
              data-canvas-message-result='video'
              src={media.videoUrl}
              poster={media.src}
              controls
              style={styles.messageResultVideo}
              onClick={() => setVideoSelectedTask(message.video_task || null)}
            />
          </div>
        );
      }
      if (media.status === 'failed') {
        return (
          <Text type='danger' style={styles.canvasErrorText}>
            {media.error || t('生成失败')}
          </Text>
        );
      }
      return (
        <div style={styles.messagePending}>
          {media.status === 'in_progress' ? <Spin size='small' /> : <IconPlayCircle />}
          <span>{media.status === 'in_progress' ? t('生成中') : t('排队中')}</span>
        </div>
      );
    }
    return null;
  };

  const renderCanvasMessage = (message) => {
    if (message?.render_type === CANVAS_RENDERABLE_IMAGE_BATCH) {
      return renderCanvasGenerationBatch(message);
    }

    const isUser = message.role === 'user';
    const references = getCanvasMessageReferenceFiles(message);
    const media = getCanvasMessageMedia(message);
    const isSelected = selectedCanvasMessageId === message.id;
    const isChatMode = generationMode === CANVAS_MODE_CHAT;
    const handleMessageClick = async () => {
      setSelectedCanvasMessageId(message.id);
      if (isUser || references.length > 0 || isChatMode) {
        return;
      }
      const detail = await loadCanvasMessageTaskDetail(message);
      if (!detail) {
        return;
      }
      setCanvasMessages((prev) =>
        prev.map((item) =>
          item.id === message.id ? mergeCanvasMessageTaskDetail(item, detail) : item,
        ),
      );
    };

    if (isChatMode) {
      return (
        <div
          key={message.id}
          data-canvas-message-row='true'
          style={{
            ...styles.canvasMessageRow,
            ...(isUser
              ? styles.canvasMessageRowUser
              : styles.canvasMessageRowAssistant),
            ...(isSelected ? styles.canvasMessageRowSelected : null),
          }}
          onClick={handleMessageClick}
        >
          <div style={styles.canvasMessageHead}>
            <Text type='tertiary' size='small'>
              {isUser ? t('你') : t('生成')}
            </Text>
          </div>
          <div style={styles.canvasMessageBody}>
            {isUser ? (
              <div style={styles.canvasMessagePrompt}>
                {message.prompt || t('已添加素材引用')}
              </div>
            ) : null}
            {references.length > 0 ? (
              <div style={styles.canvasMessageRefs}>
                {references.map((file) =>
                  renderCanvasReferenceThumb(file, t('参考图')),
                )}
              </div>
            ) : null}
            {!isUser ? (
              <div style={styles.canvasMessageResult}>
                {renderCanvasMessageResult(message)}
              </div>
            ) : null}
            {media?.status === 'failed' && message.error_message ? (
              <Text type='danger' size='small' style={styles.canvasErrorText}>
                {message.error_message}
              </Text>
            ) : null}
          </div>
        </div>
      );
    }

    if (isUser) {
      return (
        <div
          key={message.id}
          data-canvas-message-row='true'
          style={{
            ...styles.canvasMessageRow,
            ...styles.canvasMessageRowUser,
            ...(isSelected ? styles.canvasMessageRowSelected : null),
          }}
        >
          <div style={styles.canvasMessageBody}>
            <div style={styles.canvasMessagePrompt}>
              {message.prompt || t('已添加素材引用')}
            </div>
            {references.length > 0 ? (
              <div style={styles.canvasMessageRefs}>
                {references.map((file) =>
                  renderCanvasReferenceThumb(file, t('参考图')),
                )}
              </div>
            ) : null}
          </div>
        </div>
      );
    }

    return (
      <div
        key={message.id}
        data-canvas-message-row='true'
        style={{
          ...styles.canvasGenerationRow,
          ...(isSelected ? styles.canvasGenerationRowSelected : null),
        }}
        onClick={handleMessageClick}
      >
        <div style={styles.canvasGenerationCardWrap}>
          {renderCanvasGenerationCard(message)}
          {references.length > 0 ? (
            <div style={styles.canvasMessageRefs}>
              {references.map((file) =>
                renderCanvasReferenceThumb(file, t('参考图')),
              )}
            </div>
          ) : null}
        </div>
      </div>
    );
  };

  const renderCanvasMessageStream = () => {
    if (!selectedCanvasSession) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(t('空白会话'), t('准备好开始了吗？'))}
        </div>
      );
    }
    if (canvasMessagesLoading) {
      return (
        <div style={styles.canvasStreamEmpty}>
          <div style={{ padding: 24, display: 'flex', justifyContent: 'center' }}>
            <Spin />
          </div>
        </div>
      );
    }
    if (canvasMessagesError) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(
            t('消息加载失败'),
            canvasMessagesError,
            <Button size='small' onClick={() => loadCanvasMessages(selectedCanvasSession.id)}>
              {t('重试')}
            </Button>,
          )}
        </div>
      );
    }
    if (displayedCanvasMessages.length === 0) {
      return (
        <div style={styles.canvasStreamEmpty}>
          {renderSidebarEmpty(t('空白会话'), t('准备好开始了吗？'))}
        </div>
      );
    }
    return (
      <div style={styles.canvasMessageList}>
        {getRenderableCanvasMessages(displayedCanvasMessages).map(
          renderCanvasMessage,
        )}
      </div>
    );
  };

  const renderComposerModeSwitch = () => {
    const options = [
      {
        value: CANVAS_MODE_CHAT,
        label: t('文本对话'),
        icon: <IconCommentStroked size='small' />,
      },
      {
        value: CANVAS_MODE_IMAGE,
        label: t('图片生成'),
        icon: <IconImage size='small' />,
      },
      {
        value: CANVAS_MODE_VIDEO,
        label: t('视频生成'),
        icon: <IconVideo size='small' />,
      },
    ];
    const activeOption =
      options.find((option) => option.value === generationMode) || options[0];
    return (
      renderPillDropdown({
        key: 'composer-mode',
        label: '',
        icon: activeOption.icon,
        value: generationMode,
        displayValue: activeOption.label,
        onChange: handleModeChange,
        options,
      })
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
        icon: <IconLayers size='small' />,
        value: chatModel,
        displayValue: chatModel,
        onChange: (value) => {
          setChatModel(value);
          updateCurrentCanvasSessionModel(CANVAS_MODE_CHAT, value);
        },
        options: chatModels.map((model) => ({ value: model, label: model })),
        disabled: chatModels.length === 0,
      }),
      renderPillDropdown({
        key: 'chat-temperature',
        label: t('温度'),
        icon: <IconSetting size='small' />,
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
        icon: <IconText size='small' />,
        value: chatContext,
        displayValue: chatContext,
        onChange: setChatContext,
        options: ['4', '8', '16', '32'].map((item) => ({
          value: item,
          label: item,
        })),
      }),
      isMobile ? (
        <button
          key='chat-tools'
          type='button'
          aria-label={t('工具')}
          title={t('工具')}
          aria-pressed={chatToolsEnabled}
          style={{
            ...styles.pillButton,
            ...styles.pillButtonIconOnly,
            ...(chatToolsEnabled ? styles.pillButtonActive : styles.pillButtonMuted),
            cursor: 'pointer',
          }}
          onClick={() => setChatToolsEnabled((current) => !current)}
        >
          <IconSetting size='small' />
        </button>
      ) : (
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
        </label>
      ),
    ];
    const imageComposerParameters = [
      renderModelDropdown(false, activeModelLabel),
      renderImageParametersDropdown(),
      selectedModelSupportsMaskEditing &&
        referenceImages.length > 0 && (
          <button
            key='image-advanced'
            type='button'
            aria-label={t('高级')}
            title={t('高级')}
            style={{
              ...styles.pillButton,
              ...(isMobile ? styles.pillButtonIconOnly : null),
              ...(composerAdvancedVisible
                ? styles.pillButtonActive
                : styles.pillButtonMuted),
              cursor: 'pointer',
            }}
            onClick={() => setComposerAdvancedVisible((current) => !current)}
          >
            <IconSetting size='small' />
            {!isMobile ? <span>{t('高级')}</span> : null}
          </button>
        ),
    ].filter(Boolean);
    const videoComposerParameters = [
      renderModelDropdown(true, activeModelLabel),
      renderVideoParametersDropdown(),
      renderPillDropdown({
        key: 'video-duration',
        label: t('时长'),
        icon: <IconClock size='small' />,
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
            <div style={styles.promptInputRow}>
              {showPromptAssetBar ? (
                <div style={styles.promptInlineAssets}>
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
                        renderReferenceThumb(file, () =>
                          handleImageRemove(file),
                        ),
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
                    ? renderReferenceThumb(
                        videoReferenceImage,
                        handleVideoReferenceRemove,
                      )
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
            </div>
            <div style={styles.promptControls}>
              <div style={styles.promptControlsLeft}>
                {renderComposerModeSwitch()}
                <div style={styles.composerParameterRow}>{activeParameters}</div>
              </div>
              <div style={styles.promptControlsRight}>
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
                      ? 'var(--semi-color-primary)'
                      : 'var(--semi-color-fill-0)',
                    borderColor: promptHasContent
                      ? 'var(--semi-color-primary)'
                      : 'var(--semi-color-border)',
                    color: promptHasContent ? '#fff' : 'var(--semi-color-text-2)',
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
                  borderTop: '1px solid var(--semi-color-border)',
                  paddingTop: 10,
                  marginTop: 8,
                }}
              >
                <Text
                  size='small'
                  style={{
                    display: 'block',
                    marginBottom: 8,
                    color: 'var(--semi-color-text-2)',
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
    const assetId = getAssetComparableKey(asset);
    const isSelected = selectedCanvasAssetIds.has(assetId);
    const stopCardActionPropagation = (event) => {
      event.stopPropagation();
    };
    return (
      <div
        key={getAssetKey(asset)}
        style={styles.assetCard}
        data-canvas-asset-card='true'
        onClick={() => setSelectedAssetPreview(asset)}
      >
        <div style={styles.assetSelect} onClick={stopCardActionPropagation}>
          <Checkbox
            checked={isSelected}
            onChange={(event) =>
              handleCanvasAssetSelect(asset, event.target.checked)
            }
            aria-label={t('选择资产')}
          />
        </div>
        {imageUrl ? (
          <img src={imageUrl} alt='' style={styles.assetThumb} />
        ) : (
          <div style={{ ...styles.assetThumb, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <IconImage size='large' />
          </div>
        )}
        <div style={styles.assetCardMeta}>
          <Text size='small' strong ellipsis={{ rows: 1 }}>
            {getAssetDisplayName(asset)}
          </Text>
        </div>
        <div style={styles.assetActions}>
          <Button
            size='small'
            type='primary'
            onClick={(event) => {
              event.stopPropagation();
              insertAssetIntoCurrentMode(asset);
            }}
          >
            {generationMode === CANVAS_MODE_CHAT
              ? t('插入')
              : generationMode === CANVAS_MODE_VIDEO
                ? t('首帧')
                : t('参考图')}
          </Button>
          <Button
            size='small'
            aria-label={t('下载图片')}
            icon={<IconDownload />}
            onClick={(event) => {
              event.stopPropagation();
              downloadAsset(asset);
            }}
          />
          <span onClick={stopCardActionPropagation}>
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
          </span>
        </div>
      </div>
    );
  };

  const renderAssetPreviewMeta = (label, value) => (
    <div style={styles.assetPreviewMetaItem}>
      <Text type='tertiary' size='small'>
        {label}
      </Text>
      <Text size='small' style={{ wordBreak: 'break-word' }}>
        {value || '-'}
      </Text>
    </div>
  );

  const renderAssetPreviewOverlay = () => {
    if (!selectedAssetPreview) {
      return null;
    }
    const imageUrl = getAssetImageUrl(selectedAssetPreview);
    return (
      <div
        style={styles.assetPreviewOverlay}
        onClick={() => setSelectedAssetPreview(null)}
        data-canvas-asset-preview='true'
      >
        <div
          style={styles.assetPreviewPanel}
          onClick={(event) => event.stopPropagation()}
        >
          <div style={styles.assetPreviewImagePane}>
            {imageUrl ? (
              <img
                src={imageUrl}
                alt=''
                style={styles.assetPreviewImage}
              />
            ) : (
              <IconImage size='extra-large' />
            )}
          </div>
          <div style={styles.assetPreviewInfo}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ minWidth: 0 }}>
                <Text strong ellipsis={{ rows: 2 }}>
                  {getAssetDisplayName(selectedAssetPreview)}
                </Text>
                <Text
                  type='tertiary'
                  size='small'
                  ellipsis={{ rows: 1 }}
                  style={{ display: 'block', marginTop: 4 }}
                >
                  {selectedAssetPreview.model_id || '-'}
                </Text>
              </div>
              <Button
                size='small'
                type='tertiary'
                onClick={() => setSelectedAssetPreview(null)}
              >
                {t('关闭')}
              </Button>
            </div>
            {renderAssetPreviewMeta(
              t('名称'),
              getAssetDisplayName(selectedAssetPreview),
            )}
            {renderAssetPreviewMeta(
              'model_id',
              selectedAssetPreview.model_id || '-',
            )}
            {renderAssetPreviewMeta(
              t('尺寸/比例'),
              getAssetSizeText(selectedAssetPreview),
            )}
            {renderAssetPreviewMeta(
              t('任务 ID'),
              getAssetTaskIdText(selectedAssetPreview),
            )}
            {renderWeakDetails(
              t('更多信息'),
              <>
                {renderAssetPreviewMeta(
                  t('创建时间'),
                  formatTimestamp(selectedAssetPreview.created_time),
                )}
                {renderAssetPreviewMeta(
                  t('完成时间'),
                  formatTimestamp(selectedAssetPreview.completed_time),
                )}
                {renderAssetPreviewMeta(
                  t('提示词'),
                  selectedAssetPreview.prompt || '-',
                )}
              </>,
            )}
          </div>
        </div>
      </div>
    );
  };

  const renderCanvasImagePreviewOverlay = () => {
    if (!selectedCanvasImagePreview?.src) {
      return null;
    }
    return (
      <div
        style={styles.assetPreviewOverlay}
        onClick={() => setSelectedCanvasImagePreview(null)}
        data-canvas-image-preview='true'
      >
        <div
          style={styles.canvasImagePreviewPanel}
          onClick={(event) => event.stopPropagation()}
        >
          <img
            src={selectedCanvasImagePreview.src}
            alt=''
            style={styles.canvasImagePreviewImage}
          />
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
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
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
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
            <Checkbox
              checked={allCurrentCanvasAssetsSelected}
              indeterminate={partiallyCurrentCanvasAssetsSelected}
              disabled={canvasAssets.length === 0 || canvasAssetsLoading}
              onChange={(event) =>
                handleCanvasAssetSelectAll(event.target.checked)
              }
            >
              {allCurrentCanvasAssetsSelected
                ? t('取消选择当前页')
                : t('选择当前页')}
            </Checkbox>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <Tag color={selectedCanvasAssetCount > 0 ? 'blue' : 'grey'}>
                {t('已选 {{count}} 项', { count: selectedCanvasAssetCount })}
              </Tag>
              <Button
                size='small'
                type='tertiary'
                disabled={selectedCanvasAssetCount === 0 || assetBatchDeleting}
                onClick={() => setSelectedCanvasAssetIds(new Set())}
              >
                {t('取消选择')}
              </Button>
              <Popconfirm
                title={t('确认删除选中资产？')}
                content={t('删除后无法恢复，请确认是否继续')}
                okType='danger'
                onConfirm={deleteSelectedCanvasAssets}
              >
                <Button
                  size='small'
                  type='danger'
                  icon={<IconDelete />}
                  loading={assetBatchDeleting}
                  disabled={selectedCanvasAssetCount === 0}
                >
                  {t('删除选中')}
                </Button>
              </Popconfirm>
            </div>
          </div>
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
      {!isMobile && desktopSidebarCollapsed ? (
        <div style={styles.sidebarRevealButtonWrap}>
          <Tooltip content={t('显示侧边栏')} position='right'>
            <Button
              type='tertiary'
              aria-label={t('显示侧边栏')}
              data-canvas-sidebar-reveal='true'
              icon={<IconSidebar />}
              style={styles.sidebarIconButton}
              onClick={() => setDesktopSidebarCollapsed(false)}
            />
          </Tooltip>
        </div>
      ) : null}
      {isMobile ? (
        <div style={styles.workspaceTopbar}>
          <div style={styles.workspaceTopbarLeft}>
            <Button
              type='tertiary'
              aria-label={t('打开任务栏')}
              data-canvas-mobile-taskbar-trigger='true'
              icon={<IconMenu />}
              onClick={() => setMobileTaskbarVisible(true)}
            />
          </div>
        </div>
      ) : null}
      <div style={styles.workspaceBody}>
        <div ref={canvasMessageViewportRef} style={styles.mainViewport}>
          {generationMode === CANVAS_MODE_CHAT
            ? renderChatWorkspace()
            : renderCanvasMessageStream()}
        </div>
        {renderComposer()}
      </div>
    </div>
  );

  return (
    <div style={styles.container}>
      {!isMobile && !desktopSidebarCollapsed ? renderTaskSidebar() : null}
      {renderWorkspace()}
      {isMobile ? (
        <SideSheet
          visible={mobileTaskbarVisible}
          title={t('导航')}
          width='100%'
          onCancel={() => setMobileTaskbarVisible(false)}
          bodyStyle={{ padding: 0 }}
        >
          {renderTaskSidebar()}
        </SideSheet>
      ) : null}
      {renderAssetDrawer()}
      {renderAssetPreviewOverlay()}
      {renderCanvasImagePreviewOverlay()}
      <ImageGenerationTaskModal
        visible={taskModalVisible}
        task={selectedTask}
        onClose={() => setTaskModalVisible(false)}
        onRetrySuccess={(task) => {
          updateTaskInList(task);
          setSelectedTask(task);
        }}
        onDeleted={(taskId) => {
          removeImageTasksFromLocalState([taskId]);
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
          removeVideoTasksFromLocalState([taskId]);
        }}
      />
    </div>
  );
};

export default ImageGeneration;
