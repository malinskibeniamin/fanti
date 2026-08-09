const SPEECH_RATE = 0.82;

/**
 * Pronounce Chinese text with the browser TTS voice. Cancels any in-flight
 * utterance first so rapid taps never queue up. Mirrors the reader's
 * dictionary sheet: zh-TW voice at a learner-friendly rate.
 */
export function speak(text: string) {
  if (!('speechSynthesis' in window)) {
    return;
  }
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = 'zh-TW';
  utterance.rate = SPEECH_RATE;
  window.speechSynthesis.cancel();
  window.speechSynthesis.speak(utterance);
}
