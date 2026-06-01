export const THINK_SEPARATOR = '\n\n---\n\n';
export const CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS = 1000;

const CANVAS_CHAT_REASONING_FEW_SECONDS_THRESHOLD_MS = 2000;

const normalizeText = (value) => (typeof value === 'string' ? value : '');

const appendReasoningSegment = (segments, value) => {
  const text = normalizeText(value).trim();
  if (text) {
    segments.push(text);
  }
};

const normalizeCanvasChatStatus = (value) =>
  normalizeText(value).trim().toLowerCase();

const isCanvasChatReasoningTerminalStatus = (value) =>
  ['success', 'failed', 'stopped'].includes(normalizeCanvasChatStatus(value));

export const createCanvasChatReasoningUiState = (overrides = {}) => ({
  isReasoningStreaming: false,
  reasoningStartedAt: 0,
  reasoningCompletedAt: 0,
  isExpanded: false,
  hasAutoCollapsed: false,
  ...overrides,
});

export const areCanvasChatReasoningUiStatesEqual = (left, right) => {
  if (left === right) {
    return true;
  }
  if (!left || !right) {
    return false;
  }
  return (
    left.isReasoningStreaming === right.isReasoningStreaming &&
    left.reasoningStartedAt === right.reasoningStartedAt &&
    left.reasoningCompletedAt === right.reasoningCompletedAt &&
    left.isExpanded === right.isExpanded &&
    left.hasAutoCollapsed === right.hasAutoCollapsed
  );
};

export const extractCanvasChatReasoning = (message) => {
  const reasoningSegments = [];
  let displayContent = normalizeText(message?.prompt);

  appendReasoningSegment(
    reasoningSegments,
    message?.reasoning_content ?? message?.reasoningContent,
  );

  if (displayContent.includes('<think>')) {
    const thinkTagRegex = /<think>([\s\S]*?)<\/think>/g;
    const replyParts = [];
    let lastIndex = 0;
    let match;

    while ((match = thinkTagRegex.exec(displayContent)) !== null) {
      replyParts.push(displayContent.substring(lastIndex, match.index));
      appendReasoningSegment(reasoningSegments, match[1]);
      lastIndex = match.index + match[0].length;
    }

    if (lastIndex > 0) {
      replyParts.push(displayContent.substring(lastIndex));
      displayContent = replyParts.join('');
    }
  }

  const lastOpenThinkIndex = displayContent.lastIndexOf('<think>');
  if (lastOpenThinkIndex !== -1) {
    const unclosedFragment = displayContent.substring(lastOpenThinkIndex);
    if (!unclosedFragment.includes('</think>')) {
      appendReasoningSegment(
        reasoningSegments,
        unclosedFragment.substring('<think>'.length),
      );
      displayContent = displayContent.substring(0, lastOpenThinkIndex);
    }
  }

  return {
    content: displayContent.replace(/<\/?think>/g, '').trim(),
    reasoningContent: reasoningSegments.join(THINK_SEPARATOR),
    hasReasoning: reasoningSegments.length > 0,
  };
};

export const syncCanvasChatReasoningUiState = ({
  message,
  previousState,
  hasReasoning,
  isLiveMessage = false,
  now = Date.now(),
}) => {
  if (!hasReasoning) {
    return null;
  }

  const status = normalizeCanvasChatStatus(message?.status);
  const isReasoningStreaming = status === 'generating';

  if (!previousState) {
    if (isReasoningStreaming && isLiveMessage) {
      return createCanvasChatReasoningUiState({
        isReasoningStreaming: true,
        reasoningStartedAt: now,
        isExpanded: true,
      });
    }
    if (isReasoningStreaming) {
      return createCanvasChatReasoningUiState({
        isReasoningStreaming: true,
      });
    }
    return createCanvasChatReasoningUiState({
      hasAutoCollapsed: true,
    });
  }

  const nextState = {
    ...previousState,
  };

  if (isReasoningStreaming) {
    nextState.isReasoningStreaming = true;
    nextState.reasoningCompletedAt = 0;
    if (isLiveMessage && !previousState.reasoningStartedAt) {
      nextState.reasoningStartedAt = now;
      nextState.isExpanded = true;
      nextState.hasAutoCollapsed = false;
    }
    return nextState;
  }

  if (previousState.isReasoningStreaming) {
    nextState.isReasoningStreaming = false;
    if (previousState.reasoningStartedAt && !previousState.reasoningCompletedAt) {
      nextState.reasoningCompletedAt = now;
    }
    if (!previousState.isExpanded || !previousState.reasoningStartedAt) {
      nextState.hasAutoCollapsed = true;
    }
    return nextState;
  }

  if (
    isCanvasChatReasoningTerminalStatus(status) &&
    previousState.reasoningStartedAt &&
    !previousState.reasoningCompletedAt
  ) {
    nextState.reasoningCompletedAt = now;
  }

  if (isCanvasChatReasoningTerminalStatus(status) && !previousState.isExpanded) {
    nextState.hasAutoCollapsed = true;
  }

  return nextState;
};

export const getCanvasChatReasoningDurationMs = (uiState) => {
  if (
    !uiState?.reasoningStartedAt ||
    !uiState?.reasoningCompletedAt ||
    uiState.reasoningCompletedAt < uiState.reasoningStartedAt
  ) {
    return 0;
  }
  return uiState.reasoningCompletedAt - uiState.reasoningStartedAt;
};

export const getCanvasChatReasoningTriggerText = (uiState) => {
  if (uiState?.isReasoningStreaming) {
    return 'Thinking...';
  }

  const durationMs = getCanvasChatReasoningDurationMs(uiState);
  if (durationMs < CANVAS_CHAT_REASONING_FEW_SECONDS_THRESHOLD_MS) {
    return 'Thought for a few seconds';
  }

  return `Thought for ${Math.max(1, Math.round(durationMs / 1000))} seconds`;
};

export const getCanvasChatReasoningAutoCollapseRemainingMs = (
  uiState,
  now = Date.now(),
  delayMs = CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS,
) => {
  if (
    !uiState ||
    uiState.isReasoningStreaming ||
    uiState.hasAutoCollapsed ||
    !uiState.reasoningCompletedAt ||
    !uiState.isExpanded
  ) {
    return null;
  }
  return Math.max(delayMs - (now - uiState.reasoningCompletedAt), 0);
};

export const shouldAutoCollapseCanvasChatReasoning = (
  uiState,
  now = Date.now(),
  delayMs = CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS,
) => getCanvasChatReasoningAutoCollapseRemainingMs(uiState, now, delayMs) === 0;

export const shouldHideCanvasChatAssistantText = ({
  hasReasoning,
  reasoningUiState,
}) => Boolean(hasReasoning && reasoningUiState?.isReasoningStreaming);
