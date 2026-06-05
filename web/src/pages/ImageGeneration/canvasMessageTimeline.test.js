import { describe, expect, test } from 'bun:test';

import {
  applyCanvasTaskUpdates,
  buildCanvasMessageIndexes,
  buildCanvasChatRetryPromptMap,
  mergeCanvasLatestTimelinePage,
  mergeCanvasMessagesById,
  replaceCanvasMessagesByRequestId,
  updateCanvasMessageById,
  updateCanvasMessagesByTask,
  upsertCanvasMessages,
} from './canvasMessageTimeline';

describe('canvasMessageTimeline', () => {
  test('mergeCanvasMessagesById preserves chronological order for older pages', () => {
    const existingMessages = [
      { id: 3, role: 'user', prompt: 'third', created_time: 30 },
      { id: 4, role: 'assistant', prompt: 'fourth', created_time: 40 },
    ];
    const olderMessages = [
      { id: 1, role: 'user', prompt: 'first', created_time: 10 },
      { id: 2, role: 'assistant', prompt: 'second', created_time: 20 },
    ];

    const merged = mergeCanvasMessagesById(existingMessages, olderMessages);

    expect(merged.map((message) => message.id)).toEqual([1, 2, 3, 4]);
  });

  test('mergeCanvasLatestTimelinePage keeps older history while refreshing newest page', () => {
    const existingMessages = [
      { id: 1, role: 'user', prompt: 'older', created_time: 10 },
      { id: 2, role: 'assistant', prompt: 'newer-a', created_time: 20 },
      { id: 3, role: 'assistant', prompt: 'newer-b', created_time: 30 },
    ];
    const latestPageMessages = [
      { id: 2, role: 'assistant', prompt: 'newer-a updated', created_time: 20 },
      { id: 3, role: 'assistant', prompt: 'newer-b', created_time: 30 },
    ];

    const result = mergeCanvasLatestTimelinePage(
      existingMessages,
      latestPageMessages,
    );

    expect(result.hasOlderLoadedHistory).toBe(true);
    expect(result.messages.map((message) => message.id)).toEqual([1, 2, 3]);
    expect(result.messages[1].prompt).toBe('newer-a updated');
  });

  test('mergeCanvasLatestTimelinePage clears optimistic placeholders when latest page has real request messages', () => {
    const existingMessages = [
      { id: 1, role: 'user', prompt: 'older', created_time: 10 },
      {
        id: 'req-1-user',
        role: 'user',
        prompt: 'hello',
        client_request_id: 'req-1',
        created_time: 20,
      },
      {
        id: 'req-1-assistant',
        role: 'assistant',
        prompt: '',
        status: 'generating',
        client_request_id: 'req-1',
        created_time: 20,
      },
    ];
    const latestPageMessages = [
      {
        id: 2,
        role: 'user',
        prompt: 'hello',
        client_request_id: 'req-1',
        created_time: 20,
      },
      {
        id: 3,
        role: 'assistant',
        prompt: 'world',
        client_request_id: 'req-1',
        created_time: 21,
      },
    ];

    const result = mergeCanvasLatestTimelinePage(
      existingMessages,
      latestPageMessages,
    );

    expect(result.hasOlderLoadedHistory).toBe(true);
    expect(result.messages.map((message) => message.id)).toEqual([1, 2, 3]);
  });

  test('upsertCanvasMessages updates reasoning content without reordering', () => {
    const existingMessages = [
      { id: 10, role: 'user', prompt: 'hello', created_time: 10 },
      {
        id: 11,
        role: 'assistant',
        prompt: 'world',
        reasoning_content: '',
        created_time: 11,
      },
    ];

    const updatedMessages = upsertCanvasMessages(existingMessages, [
      {
        id: 11,
        role: 'assistant',
        prompt: 'world!',
        reasoning_content: 'thinking...',
        created_time: 11,
      },
      { id: 12, role: 'assistant', prompt: 'done', created_time: 12 },
    ]);

    expect(updatedMessages.map((message) => message.id)).toEqual([10, 11, 12]);
    expect(updatedMessages[1].prompt).toBe('world!');
    expect(updatedMessages[1].reasoning_content).toBe('thinking...');
  });

  test('upsertCanvasMessages replaces a lone optimistic assistant placeholder by request id', () => {
    const existingMessages = [
      {
        id: 'req-1-user',
        role: 'user',
        prompt: 'hello',
        client_request_id: 'req-1',
        created_time: 10,
      },
      {
        id: 'req-1-assistant',
        role: 'assistant',
        prompt: '',
        status: 'generating',
        client_request_id: 'req-1',
        created_time: 10,
      },
    ];

    const updatedMessages = upsertCanvasMessages(existingMessages, [
      {
        id: 22,
        role: 'assistant',
        prompt: 'partial',
        status: 'generating',
        client_request_id: 'req-1',
        created_time: 11,
      },
    ]);

    expect(updatedMessages.map((message) => message.id)).toEqual([
      'req-1-user',
      22,
    ]);
    expect(updatedMessages[1].prompt).toBe('partial');
  });

  test('updateCanvasMessageById changes only the targeted message', () => {
    const messages = [
      { id: 1, role: 'user', prompt: 'keep', created_time: 1 },
      {
        id: 2,
        role: 'assistant',
        prompt: 'reply',
        reasoning_content: '',
        created_time: 2,
      },
    ];

    const updated = updateCanvasMessageById(messages, 2, (message) => ({
      ...message,
      prompt: `${message.prompt}!`,
      reasoning_content: 'delta',
    }));

    expect(updated.map((message) => message.id)).toEqual([1, 2]);
    expect(updated[0]).toEqual(messages[0]);
    expect(updated[1].prompt).toBe('reply!');
    expect(updated[1].reasoning_content).toBe('delta');
  });

  test('updateCanvasMessagesByTask updates matching task cards without reordering', () => {
    const messages = [
      { id: 1, role: 'user', prompt: 'first', created_time: 1 },
      {
        id: 2,
        role: 'assistant',
        prompt: 'image result',
        task_id: '22',
        task_type: 'image_generation',
        status: 'pending',
        image_task: { id: 22, status: 'pending' },
        created_time: 2,
      },
      {
        id: 3,
        role: 'assistant',
        prompt: 'video result',
        task_id: '33',
        task_type: 'video_generation',
        status: 'queued',
        video_task: { id: 33, status: 'queued' },
        created_time: 3,
      },
    ];

    const updated = updateCanvasMessagesByTask(
      messages,
      'video_generation',
      33,
      (message) => ({
        ...message,
        status: 'completed',
        video_task: {
          ...(message.video_task || {}),
          status: 'completed',
          result_url: 'https://cdn.example.com/video.mp4',
        },
      }),
    );

    expect(updated.map((message) => message.id)).toEqual([1, 2, 3]);
    expect(updated[1]).toEqual(messages[1]);
    expect(updated[2].status).toBe('completed');
    expect(updated[2].video_task.result_url).toBe(
      'https://cdn.example.com/video.mp4',
    );
  });

  test('updateCanvasMessagesByTask falls back when cached task index is stale', () => {
    const messages = [
      {
        id: 1,
        role: 'assistant',
        prompt: 'video result',
        task_id: '33',
        task_type: 'video_generation',
        status: 'queued',
        video_task: { id: 33, status: 'queued' },
        created_time: 1,
      },
      {
        id: 2,
        role: 'assistant',
        prompt: 'different task',
        task_id: '44',
        task_type: 'video_generation',
        status: 'queued',
        video_task: { id: 44, status: 'queued' },
        created_time: 2,
      },
    ];
    const staleIndexes = buildCanvasMessageIndexes([messages[1], messages[0]]);

    const updated = updateCanvasMessagesByTask(
      messages,
      'video_generation',
      33,
      (message) => ({
        ...message,
        status: 'completed',
        video_task: {
          ...(message.video_task || {}),
          status: 'completed',
        },
      }),
      staleIndexes,
    );

    expect(updated[0].status).toBe('completed');
    expect(updated[1]).toEqual(messages[1]);
  });

  test('applyCanvasTaskUpdates updates multiple matching task messages in one pass', () => {
    const messages = [
      {
        id: 1,
        role: 'assistant',
        prompt: 'image result',
        task_id: '11',
        task_type: 'image_generation',
        status: 'pending',
        image_task: { id: 11, status: 'pending' },
        created_time: 1,
      },
      {
        id: 2,
        role: 'assistant',
        prompt: 'video result',
        task_id: '22',
        task_type: 'video_generation',
        status: 'queued',
        video_task: { id: 22, status: 'queued' },
        created_time: 2,
      },
      {
        id: 3,
        role: 'assistant',
        prompt: 'unrelated',
        task_id: '33',
        task_type: 'video_generation',
        status: 'queued',
        video_task: { id: 33, status: 'queued' },
        created_time: 3,
      },
    ];

    const updatedImages = applyCanvasTaskUpdates(
      messages,
      [
        {
          id: 11,
          status: 'success',
          image_url: 'https://cdn.example.com/a.png',
        },
      ],
      'image',
    );
    expect(updatedImages[0].status).toBe('success');
    expect(updatedImages[0].image_task.image_url).toBe(
      'https://cdn.example.com/a.png',
    );
    expect(updatedImages[1]).toEqual(messages[1]);

    const updatedVideos = applyCanvasTaskUpdates(
      messages,
      [
        {
          id: 22,
          status: 'completed',
          result_url: 'https://cdn.example.com/b.mp4',
        },
        { id: 33, status: 'failed', fail_reason: 'boom' },
      ],
      'video',
    );
    expect(updatedVideos[1].status).toBe('completed');
    expect(updatedVideos[1].video_task.result_url).toBe(
      'https://cdn.example.com/b.mp4',
    );
    expect(updatedVideos[2].status).toBe('failed');
    expect(updatedVideos[2].error_message).toBe('boom');
  });

  test('applyCanvasTaskUpdates returns original references when task payload is unchanged', () => {
    const messages = [
      {
        id: 1,
        role: 'assistant',
        prompt: 'video result',
        task_id: '22',
        task_type: 'video_generation',
        status: 'queued',
        error_message: '',
        video_task: {
          id: 22,
          status: 'queued',
          progress: '10%',
        },
        created_time: 1,
      },
    ];

    const updated = applyCanvasTaskUpdates(
      messages,
      [
        {
          id: 22,
          status: 'queued',
          progress: '10%',
        },
      ],
      'video',
    );

    expect(updated).toBe(messages);
    expect(updated[0]).toBe(messages[0]);
  });

  test('buildCanvasChatRetryPromptMap reuses matching user prompts', () => {
    const messages = [
      {
        id: 1,
        role: 'user',
        prompt: 'first prompt',
        client_request_id: 'req-1',
        created_time: 1,
      },
      {
        id: 2,
        role: 'assistant',
        prompt: 'first answer',
        client_request_id: 'req-1',
        created_time: 2,
      },
      {
        id: 3,
        role: 'assistant',
        prompt: 'follow-up answer',
        created_time: 3,
      },
      {
        id: 4,
        role: 'user',
        prompt: 'second prompt',
        client_request_id: 'req-2',
        created_time: 4,
      },
      {
        id: 5,
        role: 'assistant',
        prompt: 'second answer',
        client_request_id: 'req-2',
        created_time: 5,
      },
    ];

    const retryPromptMap = buildCanvasChatRetryPromptMap(messages);

    expect(retryPromptMap['2']).toBe('first prompt');
    expect(retryPromptMap['3']).toBe('first prompt');
    expect(retryPromptMap['5']).toBe('second prompt');
  });

  test('replaceCanvasMessagesByRequestId swaps optimistic chat placeholders with a server snapshot', () => {
    const messages = [
      { id: 1, role: 'user', prompt: 'older', created_time: 1 },
      {
        id: 'req-1-user',
        role: 'user',
        prompt: 'hello',
        client_request_id: 'req-1',
        created_time: 10,
      },
      {
        id: 'req-1-assistant',
        role: 'assistant',
        prompt: '',
        status: 'generating',
        client_request_id: 'req-1',
        created_time: 10,
      },
    ];

    const replaced = replaceCanvasMessagesByRequestId(messages, 'req-1', [
      {
        id: 2,
        role: 'user',
        prompt: 'hello',
        client_request_id: 'req-1',
        created_time: 10,
      },
      {
        id: 3,
        role: 'assistant',
        prompt: 'world',
        client_request_id: 'req-1',
        created_time: 11,
      },
    ]);

    expect(replaced.map((message) => message.id)).toEqual([1, 2, 3]);
  });
});
