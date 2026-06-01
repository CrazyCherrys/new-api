const normalizeCanvasMessageNumber = (value) => Number(value) || 0;

export const sortCanvasMessagesByCreated = (a, b) => {
  const createdDiff =
    normalizeCanvasMessageNumber(a?.created_time) -
    normalizeCanvasMessageNumber(b?.created_time);
  if (createdDiff !== 0) {
    return createdDiff;
  }
  return normalizeCanvasMessageNumber(a?.id) - normalizeCanvasMessageNumber(b?.id);
};

export const mergeCanvasMessagesById = (existingMessages, incomingMessages) => {
  const merged = new Map();
  (Array.isArray(existingMessages) ? existingMessages : []).forEach((message) => {
    if (message?.id) {
      merged.set(message.id, message);
      return;
    }
    merged.set(Symbol('canvas-message'), message);
  });
  (Array.isArray(incomingMessages) ? incomingMessages : []).forEach((message) => {
    if (message?.id && merged.has(message.id)) {
      merged.set(message.id, {
        ...merged.get(message.id),
        ...message,
      });
      return;
    }
    if (message?.id) {
      merged.set(message.id, message);
      return;
    }
    merged.set(Symbol('canvas-message'), message);
  });
  return Array.from(merged.values()).sort(sortCanvasMessagesByCreated);
};

export const mergeCanvasLatestTimelinePage = (
  existingMessages,
  latestPageMessages,
) => {
  const latestItems = Array.isArray(latestPageMessages) ? latestPageMessages : [];
  const existingItems = Array.isArray(existingMessages) ? existingMessages : [];
  const latestIds = new Set(
    latestItems.map((message) => message?.id).filter(Boolean),
  );
  const hasOlderLoadedHistory =
    existingItems.length > latestItems.length &&
    existingItems.some((message) => message?.id && !latestIds.has(message.id));
  return {
    messages: hasOlderLoadedHistory
      ? mergeCanvasMessagesById(existingItems, latestItems)
      : latestItems.slice().sort(sortCanvasMessagesByCreated),
    hasOlderLoadedHistory,
  };
};
