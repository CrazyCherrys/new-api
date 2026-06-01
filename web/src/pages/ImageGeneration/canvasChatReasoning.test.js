import { describe, expect, test } from 'bun:test';
import {
  CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS,
  createCanvasChatReasoningUiState,
  extractCanvasChatReasoning,
  getCanvasChatReasoningTriggerText,
  shouldAutoCollapseCanvasChatReasoning,
  shouldHideCanvasChatAssistantText,
  syncCanvasChatReasoningUiState,
} from './canvasChatReasoning';

describe('canvas chat reasoning extraction', () => {
  test('uses persisted reasoning_content when present', () => {
    const result = extractCanvasChatReasoning({
      prompt: '最终答复',
      reasoning_content: '第一步\n第二步',
    });

    expect(result).toEqual({
      content: '最终答复',
      reasoningContent: '第一步\n第二步',
      hasReasoning: true,
    });
  });

  test('extracts paired think tags from prompt content', () => {
    const result = extractCanvasChatReasoning({
      prompt: '前言<think>思考过程</think>最终答复',
    });

    expect(result).toEqual({
      content: '前言最终答复',
      reasoningContent: '思考过程',
      hasReasoning: true,
    });
  });

  test('extracts unclosed think content from streaming prompt', () => {
    const result = extractCanvasChatReasoning({
      prompt: '最终答复<think>还在思考',
    });

    expect(result).toEqual({
      content: '最终答复',
      reasoningContent: '还在思考',
      hasReasoning: true,
    });
  });

  test('merges reasoning_content with think-tag content in order', () => {
    const result = extractCanvasChatReasoning({
      prompt: '前言<think>标签思考</think>最终答复<think>流式尾巴',
      reasoning_content: '持久化思考',
    });

    expect(result).toEqual({
      content: '前言最终答复',
      reasoningContent: '持久化思考\n\n---\n\n标签思考\n\n---\n\n流式尾巴',
      hasReasoning: true,
    });
  });

  test('returns plain content when no reasoning exists', () => {
    const result = extractCanvasChatReasoning({
      prompt: '只有正文',
    });

    expect(result).toEqual({
      content: '只有正文',
      reasoningContent: '',
      hasReasoning: false,
    });
  });

  test('expands when a live message receives its first reasoning delta', () => {
    const nextState = syncCanvasChatReasoningUiState({
      message: {
        status: 'generating',
      },
      previousState: null,
      hasReasoning: true,
      isLiveMessage: true,
      now: 1200,
    });

    expect(nextState).toEqual({
      isReasoningStreaming: true,
      reasoningStartedAt: 1200,
      reasoningCompletedAt: 0,
      isExpanded: true,
      hasAutoCollapsed: false,
    });
  });

  test('does not mark reasoning complete when text and reasoning deltas overlap during streaming', () => {
    const nextState = syncCanvasChatReasoningUiState({
      message: {
        status: 'generating',
        prompt: 'partial answer',
      },
      previousState: createCanvasChatReasoningUiState({
        isReasoningStreaming: true,
        reasoningStartedAt: 1000,
        isExpanded: true,
      }),
      hasReasoning: true,
      isLiveMessage: true,
      now: 1600,
    });

    expect(nextState).toEqual({
      isReasoningStreaming: true,
      reasoningStartedAt: 1000,
      reasoningCompletedAt: 0,
      isExpanded: true,
      hasAutoCollapsed: false,
    });
  });

  test('auto-collapse becomes eligible after completion once the delay elapses', () => {
    const completedState = syncCanvasChatReasoningUiState({
      message: {
        status: 'success',
      },
      previousState: createCanvasChatReasoningUiState({
        isReasoningStreaming: true,
        reasoningStartedAt: 1000,
        isExpanded: true,
      }),
      hasReasoning: true,
      now: 1800,
    });

    expect(completedState).toEqual({
      isReasoningStreaming: false,
      reasoningStartedAt: 1000,
      reasoningCompletedAt: 1800,
      isExpanded: true,
      hasAutoCollapsed: false,
    });
    expect(
      shouldAutoCollapseCanvasChatReasoning(
        completedState,
        1800 + CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS - 1,
      ),
    ).toBe(false);
    expect(
      shouldAutoCollapseCanvasChatReasoning(
        completedState,
        1800 + CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS,
      ),
    ).toBe(true);
  });

  test('error completion also finalizes reasoning for the one-shot auto-collapse path', () => {
    const failedState = syncCanvasChatReasoningUiState({
      message: {
        status: 'failed',
      },
      previousState: createCanvasChatReasoningUiState({
        isReasoningStreaming: true,
        reasoningStartedAt: 2200,
        isExpanded: true,
      }),
      hasReasoning: true,
      now: 3200,
    });

    expect(failedState).toEqual({
      isReasoningStreaming: false,
      reasoningStartedAt: 2200,
      reasoningCompletedAt: 3200,
      isExpanded: true,
      hasAutoCollapsed: false,
    });
  });

  test('auto-collapse only triggers once', () => {
    const state = createCanvasChatReasoningUiState({
      reasoningStartedAt: 1000,
      reasoningCompletedAt: 2000,
      isExpanded: true,
      hasAutoCollapsed: true,
    });

    expect(
      shouldAutoCollapseCanvasChatReasoning(
        state,
        2000 + CANVAS_CHAT_REASONING_AUTO_COLLAPSE_DELAY_MS + 50,
      ),
    ).toBe(false);
  });

  test('historical completed messages stay collapsed by default', () => {
    const state = syncCanvasChatReasoningUiState({
      message: {
        status: 'success',
      },
      previousState: null,
      hasReasoning: true,
      isLiveMessage: false,
      now: 1500,
    });

    expect(state).toEqual({
      isReasoningStreaming: false,
      reasoningStartedAt: 0,
      reasoningCompletedAt: 0,
      isExpanded: false,
      hasAutoCollapsed: true,
    });
  });

  test('switches trigger copy between thinking, few seconds, and explicit seconds', () => {
    expect(
      getCanvasChatReasoningTriggerText(
        createCanvasChatReasoningUiState({
          isReasoningStreaming: true,
        }),
      ),
    ).toBe('Thinking...');

    expect(
      getCanvasChatReasoningTriggerText(
        createCanvasChatReasoningUiState({
          reasoningStartedAt: 1000,
          reasoningCompletedAt: 2500,
        }),
      ),
    ).toBe('Thought for a few seconds');

    expect(
      getCanvasChatReasoningTriggerText(
        createCanvasChatReasoningUiState({
          reasoningStartedAt: 1000,
          reasoningCompletedAt: 6200,
        }),
      ),
    ).toBe('Thought for 5 seconds');
  });

  test('hides assistant text while reasoning is still streaming and restores it afterwards', () => {
    expect(
      shouldHideCanvasChatAssistantText({
        hasReasoning: true,
        reasoningUiState: createCanvasChatReasoningUiState({
          isReasoningStreaming: true,
        }),
      }),
    ).toBe(true);

    expect(
      shouldHideCanvasChatAssistantText({
        hasReasoning: true,
        reasoningUiState: createCanvasChatReasoningUiState({
          isReasoningStreaming: false,
        }),
      }),
    ).toBe(false);
  });
});
