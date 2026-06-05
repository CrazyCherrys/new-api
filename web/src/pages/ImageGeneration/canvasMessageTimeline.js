const normalizeCanvasMessageNumber = (value) => Number(value) || 0;

const normalizeCanvasMessageId = (value) => {
  if (value === undefined || value === null || value === '') {
    return '';
  }
  return String(value);
};

const normalizeCanvasMessageRequestId = (value) => String(value || '').trim();
const normalizeCanvasMessageRole = (value) =>
  String(value || '')
    .trim()
    .toLowerCase();

const inferCanvasMessageTaskType = (message) =>
  message?.task_type ||
  (message?.video_task
    ? 'video_generation'
    : message?.image_task
      ? 'image_generation'
      : '');

const buildCanvasMessageTaskLookupKey = (taskType, taskId) => {
  const normalizedTaskType = String(taskType || '').trim();
  const normalizedTaskId = normalizeCanvasMessageId(taskId).trim();
  if (!normalizedTaskType || !normalizedTaskId) {
    return '';
  }
  return `${normalizedTaskType}:${normalizedTaskId}`;
};

const getCanvasTaskLookupKeyForModeAndTask = (mode, task) => {
  const taskId = task?.id;
  if (!taskId) {
    return '';
  }
  return buildCanvasMessageTaskLookupKey(
    mode === 'video' ? 'video_generation' : 'image_generation',
    taskId,
  );
};

const getCanvasMessageTaskLookupKey = (message) =>
  buildCanvasMessageTaskLookupKey(
    inferCanvasMessageTaskType(message),
    message?.task_id,
  );

const mergeCanvasTaskObject = (existingTask, updatedTask) => {
  const normalizedUpdatedTask =
    updatedTask && typeof updatedTask === 'object' ? updatedTask : null;
  if (!normalizedUpdatedTask) {
    return existingTask;
  }

  const baseTask =
    existingTask && typeof existingTask === 'object' ? existingTask : null;
  let changed = !baseTask;
  const nextTask = {
    ...(baseTask || {}),
  };

  Object.entries(normalizedUpdatedTask).forEach(([key, value]) => {
    if (nextTask[key] === value) {
      return;
    }
    nextTask[key] = value;
    changed = true;
  });

  return changed ? nextTask : existingTask;
};

const mergeCanvasTaskIntoMessage = (message, updatedTask, mode) => {
  const nextStatus = updatedTask?.status ?? message?.status;
  const nextErrorMessage =
    updatedTask?.error_message ||
    updatedTask?.fail_reason ||
    message?.error_message;
  const taskKey = mode === 'video' ? 'video_task' : 'image_task';
  const existingTask = message?.[taskKey];
  const nextTask = mergeCanvasTaskObject(existingTask, updatedTask);
  const normalizedCurrentErrorMessage = message?.error_message || '';
  const normalizedNextErrorMessage = nextErrorMessage || '';

  if (
    nextStatus === message?.status &&
    normalizedNextErrorMessage === normalizedCurrentErrorMessage &&
    nextTask === existingTask
  ) {
    return message;
  }

  if (mode === 'video') {
    return {
      ...message,
      status: nextStatus,
      error_message: normalizedNextErrorMessage,
      video_task: nextTask,
    };
  }

  return {
    ...message,
    status: nextStatus,
    error_message: normalizedNextErrorMessage,
    image_task: nextTask,
  };
};

const mergeCanvasMessageRecord = (existingMessage, incomingMessage) => {
  if (!existingMessage) {
    return incomingMessage;
  }
  return {
    ...existingMessage,
    ...incomingMessage,
  };
};

const insertCanvasMessageSorted = (messages, message) => {
  const items = Array.isArray(messages) ? messages : [];
  if (items.length === 0) {
    return [message];
  }

  if (sortCanvasMessagesByCreated(items[items.length - 1], message) <= 0) {
    return [...items, message];
  }
  if (sortCanvasMessagesByCreated(message, items[0]) <= 0) {
    return [message, ...items];
  }

  let left = 0;
  let right = items.length;
  while (left < right) {
    const middle = Math.floor((left + right) / 2);
    if (sortCanvasMessagesByCreated(items[middle], message) <= 0) {
      left = middle + 1;
    } else {
      right = middle;
    }
  }
  return [...items.slice(0, left), message, ...items.slice(left)];
};

export const sortCanvasMessagesByCreated = (a, b) => {
  const createdDiff =
    normalizeCanvasMessageNumber(a?.created_time) -
    normalizeCanvasMessageNumber(b?.created_time);
  if (createdDiff !== 0) {
    return createdDiff;
  }
  return (
    normalizeCanvasMessageNumber(a?.id) - normalizeCanvasMessageNumber(b?.id)
  );
};

export const buildCanvasMessageIndexes = (messages) => {
  const indexById = new Map();
  const taskIndexByKey = new Map();

  (Array.isArray(messages) ? messages : []).forEach((message, index) => {
    const messageId = normalizeCanvasMessageId(message?.id);
    if (messageId) {
      indexById.set(messageId, index);
    }

    const taskKey = getCanvasMessageTaskLookupKey(message);
    if (!taskKey) {
      return;
    }
    const existingIndices = taskIndexByKey.get(taskKey);
    if (existingIndices) {
      existingIndices.push(index);
      return;
    }
    taskIndexByKey.set(taskKey, [index]);
  });

  return {
    indexById,
    taskIndexByKey,
  };
};

export const upsertCanvasMessages = (
  existingMessages,
  incomingMessages,
  indexes,
) => {
  const baseItems = Array.isArray(existingMessages) ? existingMessages : [];
  const updates = Array.isArray(incomingMessages) ? incomingMessages : [];
  if (updates.length === 0) {
    return baseItems;
  }

  let nextItems = baseItems;
  let changed = false;
  let shouldResort = false;
  const indexById =
    indexes?.indexById instanceof Map ? indexes.indexById : null;

  updates.forEach((message) => {
    const messageId = normalizeCanvasMessageId(message?.id);
    if (!messageId) {
      nextItems = insertCanvasMessageSorted(
        changed ? nextItems : nextItems.slice(),
        message,
      );
      changed = true;
      return;
    }

    let index = -1;
    if (!changed && indexById) {
      const hintedIndex = indexById.get(messageId);
      if (
        Number.isInteger(hintedIndex) &&
        normalizeCanvasMessageId(nextItems[hintedIndex]?.id) === messageId
      ) {
        index = hintedIndex;
      }
    }
    if (index < 0) {
      index = nextItems.findIndex(
        (item) => normalizeCanvasMessageId(item?.id) === messageId,
      );
    }

    if (index < 0) {
      const requestId = normalizeCanvasMessageRequestId(
        message?.client_request_id,
      );
      const role = normalizeCanvasMessageRole(message?.role);
      if (requestId && role) {
        const requestMatchIndices = nextItems.reduce((result, item, itemIndex) => {
          if (
            normalizeCanvasMessageRequestId(item?.client_request_id) ===
              requestId &&
            normalizeCanvasMessageRole(item?.role) === role
          ) {
            result.push(itemIndex);
          }
          return result;
        }, []);
        if (requestMatchIndices.length === 1) {
          index = requestMatchIndices[0];
        }
      }
    }

    if (index < 0) {
      nextItems = insertCanvasMessageSorted(
        changed ? nextItems : nextItems.slice(),
        message,
      );
      changed = true;
      return;
    }

    const previousMessage = nextItems[index];
    const mergedMessage = mergeCanvasMessageRecord(previousMessage, message);
    if (!changed) {
      nextItems = nextItems.slice();
      changed = true;
    }
    nextItems[index] = mergedMessage;
    if (
      normalizeCanvasMessageNumber(previousMessage?.created_time) !==
        normalizeCanvasMessageNumber(mergedMessage?.created_time) ||
      normalizeCanvasMessageNumber(previousMessage?.id) !==
        normalizeCanvasMessageNumber(mergedMessage?.id)
    ) {
      shouldResort = true;
    }
  });

  if (!changed) {
    return baseItems;
  }
  return shouldResort
    ? nextItems.slice().sort(sortCanvasMessagesByCreated)
    : nextItems;
};

export const updateCanvasMessageById = (
  messages,
  messageId,
  updater,
  indexes,
) => {
  const items = Array.isArray(messages) ? messages : [];
  const normalizedMessageId = normalizeCanvasMessageId(messageId);
  if (!normalizedMessageId || typeof updater !== 'function') {
    return items;
  }

  let index = -1;
  const indexById =
    indexes?.indexById instanceof Map ? indexes.indexById : null;
  if (indexById) {
    const hintedIndex = indexById.get(normalizedMessageId);
    if (
      Number.isInteger(hintedIndex) &&
      normalizeCanvasMessageId(items[hintedIndex]?.id) === normalizedMessageId
    ) {
      index = hintedIndex;
    }
  }
  if (index < 0) {
    index = items.findIndex(
      (message) =>
        normalizeCanvasMessageId(message?.id) === normalizedMessageId,
    );
  }
  if (index < 0) {
    return items;
  }

  const nextMessage = updater(items[index]);
  if (!nextMessage || nextMessage === items[index]) {
    return items;
  }

  const nextItems = items.slice();
  nextItems[index] = nextMessage;
  return nextItems;
};

export const updateCanvasMessagesByTask = (
  messages,
  taskType,
  taskId,
  updater,
  indexes,
) => {
  const items = Array.isArray(messages) ? messages : [];
  const taskKey = buildCanvasMessageTaskLookupKey(taskType, taskId);
  if (!taskKey || typeof updater !== 'function') {
    return items;
  }

  let targetIndices =
    indexes?.taskIndexByKey instanceof Map
      ? indexes.taskIndexByKey.get(taskKey) || []
      : [];
  targetIndices = targetIndices.filter(
    (index) => getCanvasMessageTaskLookupKey(items[index]) === taskKey,
  );
  if (targetIndices.length === 0) {
    targetIndices = items.reduce((result, message, index) => {
      if (getCanvasMessageTaskLookupKey(message) === taskKey) {
        result.push(index);
      }
      return result;
    }, []);
  }
  if (targetIndices.length === 0) {
    return items;
  }

  let nextItems = items;
  let changed = false;
  targetIndices.forEach((index) => {
    const currentMessage = nextItems[index];
    if (!currentMessage) {
      return;
    }
    const nextMessage = updater(currentMessage);
    if (!nextMessage || nextMessage === currentMessage) {
      return;
    }
    if (!changed) {
      nextItems = items.slice();
      changed = true;
    }
    nextItems[index] = nextMessage;
  });

  return changed ? nextItems : items;
};

export const applyCanvasTaskUpdates = (messages, updates, mode) => {
  const items = Array.isArray(messages) ? messages : [];
  const taskUpdates = Array.isArray(updates) ? updates : [];
  if (items.length === 0 || taskUpdates.length === 0) {
    return items;
  }

  const taskUpdateByKey = new Map();
  taskUpdates.forEach((task) => {
    const taskKey = getCanvasTaskLookupKeyForModeAndTask(mode, task);
    if (!taskKey) {
      return;
    }
    taskUpdateByKey.set(taskKey, task);
  });
  if (taskUpdateByKey.size === 0) {
    return items;
  }

  let changed = false;
  const nextItems = items.map((message) => {
    const taskKey = getCanvasMessageTaskLookupKey(message);
    if (!taskKey || !taskUpdateByKey.has(taskKey)) {
      return message;
    }
    const nextMessage = mergeCanvasTaskIntoMessage(
      message,
      taskUpdateByKey.get(taskKey),
      mode,
    );
    if (nextMessage !== message) {
      changed = true;
    }
    return nextMessage;
  });

  return changed ? nextItems : items;
};

export const replaceCanvasMessagesByRequestId = (
  existingMessages,
  requestId,
  incomingMessages,
) => {
  const items = Array.isArray(existingMessages) ? existingMessages : [];
  const nextMessages = Array.isArray(incomingMessages) ? incomingMessages : [];
  const normalizedRequestId = normalizeCanvasMessageRequestId(requestId);
  if (!normalizedRequestId) {
    return items;
  }

  const filteredItems = [];
  let insertionIndex = -1;
  items.forEach((message) => {
    if (
      normalizeCanvasMessageRequestId(message?.client_request_id) ===
      normalizedRequestId
    ) {
      if (insertionIndex < 0) {
        insertionIndex = filteredItems.length;
      }
      return;
    }
    filteredItems.push(message);
  });

  if (insertionIndex < 0) {
    return [...filteredItems, ...nextMessages];
  }
  return [
    ...filteredItems.slice(0, insertionIndex),
    ...nextMessages,
    ...filteredItems.slice(insertionIndex),
  ];
};

export const removeCanvasMessagesByRequestId = (messages, requestId) => {
  const items = Array.isArray(messages) ? messages : [];
  const normalizedRequestId = normalizeCanvasMessageRequestId(requestId);
  if (!normalizedRequestId) {
    return items;
  }
  return items.filter(
    (message) =>
      normalizeCanvasMessageRequestId(message?.client_request_id) !==
      normalizedRequestId,
  );
};

export const buildCanvasChatRetryPromptMap = (messages) => {
  const retryPromptByMessageId = {};
  const promptByRequestId = new Map();
  let lastUserPrompt = '';

  (Array.isArray(messages) ? messages : []).forEach((message) => {
    const prompt = String(message?.prompt || '').trim();
    const requestId = normalizeCanvasMessageRequestId(
      message?.client_request_id,
    );

    if (message?.role === 'user') {
      if (prompt) {
        lastUserPrompt = prompt;
      }
      if (requestId && prompt) {
        promptByRequestId.set(requestId, prompt);
      }
      return;
    }

    const messageId = normalizeCanvasMessageId(message?.id);
    if (!messageId) {
      return;
    }

    if (requestId && promptByRequestId.has(requestId)) {
      retryPromptByMessageId[messageId] = promptByRequestId.get(requestId);
      return;
    }
    if (lastUserPrompt) {
      retryPromptByMessageId[messageId] = lastUserPrompt;
    }
  });

  return retryPromptByMessageId;
};

export const mergeCanvasMessagesById = (existingMessages, incomingMessages) => {
  const existingItems = Array.isArray(existingMessages) ? existingMessages : [];
  const incomingItems = Array.isArray(incomingMessages) ? incomingMessages : [];
  if (existingItems.length === 0) {
    return incomingItems.slice().sort(sortCanvasMessagesByCreated);
  }
  if (incomingItems.length === 0) {
    return existingItems;
  }

  const mergedById = new Map();
  existingItems.forEach((message) => {
    const messageId = normalizeCanvasMessageId(message?.id);
    if (messageId) {
      mergedById.set(messageId, message);
    }
  });
  incomingItems.forEach((message) => {
    const messageId = normalizeCanvasMessageId(message?.id);
    if (!messageId) {
      return;
    }
    mergedById.set(
      messageId,
      mergeCanvasMessageRecord(mergedById.get(messageId), message),
    );
  });

  const combined = [];
  const seenIds = new Set();
  let existingIndex = 0;
  let incomingIndex = 0;

  const pushMessage = (message) => {
    const messageId = normalizeCanvasMessageId(message?.id);
    if (!messageId) {
      combined.push(message);
      return;
    }
    if (seenIds.has(messageId)) {
      return;
    }
    seenIds.add(messageId);
    combined.push(mergedById.get(messageId) || message);
  };

  while (
    existingIndex < existingItems.length ||
    incomingIndex < incomingItems.length
  ) {
    if (existingIndex >= existingItems.length) {
      pushMessage(incomingItems[incomingIndex]);
      incomingIndex += 1;
      continue;
    }
    if (incomingIndex >= incomingItems.length) {
      pushMessage(existingItems[existingIndex]);
      existingIndex += 1;
      continue;
    }

    const existingMessage = existingItems[existingIndex];
    const incomingMessage = incomingItems[incomingIndex];
    if (sortCanvasMessagesByCreated(existingMessage, incomingMessage) <= 0) {
      pushMessage(existingMessage);
      existingIndex += 1;
      continue;
    }
    pushMessage(incomingMessage);
    incomingIndex += 1;
  }

  return combined;
};

export const mergeCanvasLatestTimelinePage = (
  existingMessages,
  latestPageMessages,
) => {
  const latestItems = Array.isArray(latestPageMessages)
    ? latestPageMessages
    : [];
  const existingItems = Array.isArray(existingMessages) ? existingMessages : [];
  const latestRequestIds = new Set(
    latestItems
      .map((message) =>
        normalizeCanvasMessageRequestId(message?.client_request_id),
      )
      .filter(Boolean),
  );
  const reconciledExistingItems =
    latestRequestIds.size === 0
      ? existingItems
      : existingItems.filter(
          (message) =>
            !latestRequestIds.has(
              normalizeCanvasMessageRequestId(message?.client_request_id),
            ),
        );
  const latestIds = new Set(
    latestItems
      .map((message) => normalizeCanvasMessageId(message?.id))
      .filter(Boolean),
  );
  const hasOlderLoadedHistory =
    reconciledExistingItems.some((message) => {
      const messageId = normalizeCanvasMessageId(message?.id);
      return messageId && !latestIds.has(messageId);
    });
  return {
    messages: hasOlderLoadedHistory
      ? mergeCanvasMessagesById(reconciledExistingItems, latestItems)
      : latestItems.slice().sort(sortCanvasMessagesByCreated),
    hasOlderLoadedHistory,
  };
};
