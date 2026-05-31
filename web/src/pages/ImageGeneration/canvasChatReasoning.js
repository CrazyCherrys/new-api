const THINK_SEPARATOR = '\n\n---\n\n';

const normalizeText = (value) => (typeof value === 'string' ? value : '');

const appendReasoningSegment = (segments, value) => {
  const text = normalizeText(value).trim();
  if (text) {
    segments.push(text);
  }
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
