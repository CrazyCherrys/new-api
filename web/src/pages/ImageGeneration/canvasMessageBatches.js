export const CANVAS_RENDERABLE_IMAGE_BATCH = 'image_generation_batch';

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

const getCanvasMessageRequestId = (message) =>
  String(message?.client_request_id || '').trim();

const isImageGenerationAssistantMessage = (message) =>
  !!message &&
  message.role !== 'user' &&
  getCanvasMessageTaskType(message) === 'image_generation';

const promptsMatch = (message, prompt) => {
  if (!prompt) {
    return true;
  }
  return getCanvasMessagePromptText(message) === prompt;
};

const canMergeByRequestId = (expectedRequestId, candidateRequestId) => {
  if (expectedRequestId || candidateRequestId) {
    return !!expectedRequestId && expectedRequestId === candidateRequestId;
  }
  return true;
};

const canvasMessageMatchesImageBatch = (message, prompt, requestId) =>
  isImageGenerationAssistantMessage(message) &&
  promptsMatch(message, prompt) &&
  canMergeByRequestId(requestId, getCanvasMessageRequestId(message));

const canvasUserMessageMatchesImageBatch = (message, prompt, requestId) =>
  !!message &&
  message.role === 'user' &&
  promptsMatch(message, prompt) &&
  canMergeByRequestId(requestId, getCanvasMessageRequestId(message));

export const getRenderableCanvasMessages = (messages) => {
  const renderableMessages = [];
  const sourceMessages = Array.isArray(messages) ? messages : [];
  let index = 0;

  while (index < sourceMessages.length) {
    const userMessage = sourceMessages[index];
    if (userMessage?.role !== 'user') {
      renderableMessages.push(userMessage);
      index += 1;
      continue;
    }

    const prompt = getCanvasMessagePromptText(userMessage);
    const requestId = getCanvasMessageRequestId(userMessage);
    const assistantMessages = [];
    let cursor = index + 1;

    while (cursor < sourceMessages.length) {
      const currentMessage = sourceMessages[cursor];
      if (canvasMessageMatchesImageBatch(currentMessage, prompt, requestId)) {
        assistantMessages.push(currentMessage);
        cursor += 1;
        continue;
      }

      const nextMessage = sourceMessages[cursor + 1];
      const nextRequestId = getCanvasMessageRequestId(currentMessage);
      if (
        canvasUserMessageMatchesImageBatch(currentMessage, prompt, requestId) &&
        canvasMessageMatchesImageBatch(nextMessage, prompt, nextRequestId)
      ) {
        assistantMessages.push(nextMessage);
        cursor += 2;
        continue;
      }

      break;
    }

    if (assistantMessages.length > 1) {
      renderableMessages.push({
        render_type: CANVAS_RENDERABLE_IMAGE_BATCH,
        id: `image-batch-${requestId || userMessage.id || index}`,
        userMessage,
        assistantMessages,
      });
      index = cursor;
      continue;
    }

    renderableMessages.push(userMessage);
    index += 1;
  }

  return renderableMessages;
};
