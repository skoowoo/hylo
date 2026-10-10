// Typing rhythm for the agent: short bursts at an uneven pace, with pauses that follow the
// structure of the text (longest between paragraphs) and the odd moment of hesitation.
const REF_PER_CHAR = 1000 / 70;

const SENTENCE_END = '。！？!?.';
const CJK_SENTENCE_END = '。！？';
const CLAUSE_END = '，,;；:：、';

export function createRhythm(perChar, random = Math.random) {
  // Slower typing pauses for longer, faster typing for less.
  const scale = Math.min(2.5, Math.max(0.5, perChar / REF_PER_CHAR));
  const between = (a, b) => (a + random() * (b - a)) * scale;
  const burstMax = Math.max(1, Math.round(60 / perChar));

  return {
    burstSize: () => 1 + Math.floor(random() * burstMax),
    gap: (chars) => chars * perChar * (0.6 + random() * 0.9),
    hesitation: () => (random() < 0.012 ? between(180, 420) : 0),
    beat: () => between(250, 450),

    // Pause owed after the text before `end` has been typed.
    pause(src, end) {
      const last = src[end - 1];
      const next = src[end];
      if (last === '\n') {
        if (next === '\n') return 0;
        if (src[end - 2] === '\n') return between(350, 700);
        const lineStart = src.lastIndexOf('\n', end - 2) + 1;
        return src[lineStart] === '#' ? between(300, 500) : between(140, 300);
      }
      if (next === '\n') return 0;
      if (SENTENCE_END.includes(last) && (CJK_SENTENCE_END.includes(last) || next === undefined || /\s/.test(next))) {
        return between(110, 260);
      }
      if (CLAUSE_END.includes(last)) return between(40, 120);
      if (last === ' ' && random() < 0.12) return between(30, 100);
      return 0;
    },
  };
}
