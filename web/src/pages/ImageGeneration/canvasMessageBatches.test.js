import { describe, expect, test } from 'bun:test';
import {
  CANVAS_RENDERABLE_IMAGE_BATCH,
  getRenderableCanvasMessages,
} from './canvasMessageBatches';

const createUserMessage = (id, prompt, clientRequestId = '') => ({
  id: `user-${id}`,
  role: 'user',
  prompt,
  client_request_id: clientRequestId,
});

const createImageMessage = (id, prompt, clientRequestId = '') => ({
  id: `assistant-${id}`,
  role: 'assistant',
  prompt,
  client_request_id: clientRequestId,
  task_type: 'image_generation',
  image_task: {
    id: `task-${id}`,
    prompt,
    status: 'success',
  },
});

const createVideoMessage = (id, prompt, clientRequestId = '') => ({
  id: `video-${id}`,
  role: 'assistant',
  prompt,
  client_request_id: clientRequestId,
  task_type: 'video_generation',
  video_task: {
    id: `video-task-${id}`,
    prompt,
    status: 'queued',
  },
});

describe('canvas message batches', () => {
  test('same prompt and same client_request_id merge into one image batch', () => {
    const messages = [];
    for (let index = 1; index <= 6; index += 1) {
      messages.push(
        createUserMessage(index, '太阳', 'req-sun'),
        createImageMessage(index, '太阳', 'req-sun'),
      );
    }

    const renderable = getRenderableCanvasMessages(messages);

    expect(renderable).toHaveLength(1);
    expect(renderable[0].render_type).toBe(CANVAS_RENDERABLE_IMAGE_BATCH);
    expect(renderable[0].assistantMessages).toHaveLength(6);
  });

  test('same prompt and different client_request_id split into separate batches', () => {
    const messages = [];
    for (let index = 1; index <= 6; index += 1) {
      messages.push(
        createUserMessage(`a-${index}`, '太阳', 'req-a'),
        createImageMessage(`a-${index}`, '太阳', 'req-a'),
      );
    }
    for (let index = 1; index <= 6; index += 1) {
      messages.push(
        createUserMessage(`b-${index}`, '太阳', 'req-b'),
        createImageMessage(`b-${index}`, '太阳', 'req-b'),
      );
    }

    const renderable = getRenderableCanvasMessages(messages);

    expect(renderable).toHaveLength(2);
    expect(renderable.every((item) => item.render_type === CANVAS_RENDERABLE_IMAGE_BATCH)).toBe(true);
    expect(renderable.map((item) => item.assistantMessages.length)).toEqual([6, 6]);
  });

  test('legacy messages without client_request_id still use prompt fallback grouping', () => {
    const messages = [
      createUserMessage(1, '太阳'),
      createImageMessage(1, '太阳'),
      createUserMessage(2, '太阳'),
      createImageMessage(2, '太阳'),
      createUserMessage(3, '太阳'),
      createImageMessage(3, '太阳'),
    ];

    const renderable = getRenderableCanvasMessages(messages);

    expect(renderable).toHaveLength(1);
    expect(renderable[0].render_type).toBe(CANVAS_RENDERABLE_IMAGE_BATCH);
    expect(renderable[0].assistantMessages).toHaveLength(3);
  });

  test('quantity 1 remains ungrouped', () => {
    const renderable = getRenderableCanvasMessages([
      createUserMessage(1, '太阳', 'req-single'),
      createImageMessage(1, '太阳', 'req-single'),
    ]);

    expect(renderable).toHaveLength(2);
    expect(renderable[0].role).toBe('user');
    expect(renderable[1].task_type).toBe('image_generation');
  });

  test('different prompts do not merge', () => {
    const renderable = getRenderableCanvasMessages([
      createUserMessage(1, '太阳', 'req-sun'),
      createImageMessage(1, '太阳', 'req-sun'),
      createUserMessage(2, '月亮', 'req-moon'),
      createImageMessage(2, '月亮', 'req-moon'),
    ]);

    expect(renderable).toHaveLength(4);
    expect(renderable.every((item) => item.render_type !== CANVAS_RENDERABLE_IMAGE_BATCH)).toBe(true);
  });

  test('video messages are not grouped into image batches', () => {
    const renderable = getRenderableCanvasMessages([
      createUserMessage(1, '太阳', 'video-req-1'),
      createVideoMessage(1, '太阳', 'video-req-1'),
      createUserMessage(2, '太阳', 'video-req-2'),
      createVideoMessage(2, '太阳', 'video-req-2'),
    ]);

    expect(renderable).toHaveLength(4);
    expect(renderable.every((item) => item.render_type !== CANVAS_RENDERABLE_IMAGE_BATCH)).toBe(true);
  });
});
