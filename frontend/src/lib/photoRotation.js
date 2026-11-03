const normalize = (degrees) => ((degrees % 360) + 360) % 360;

export function createPhotoRotation({ photo, save, onChange, onSaved, delay = 500 }) {
  let current = photo;
  let queued = 0;
  let preview = 0;
  let timer;
  let flight = false;
  let error = null;
  let disposed = false;
  const notify = () => {
    if (!disposed) onChange({ preview, busy: flight || queued !== 0, error });
  };
  function finish() {
    if (!flight && !queued) {
      preview = 0;
      onSaved(current);
    }
    notify();
  }
  async function flush() {
    clearTimeout(timer);
    timer = undefined;
    if (flight || !queued) return;
    const degrees = queued === 270 ? -90 : queued;
    queued = 0;
    flight = true;
    error = null;
    notify();
    try {
      current = await save(current, degrees);
    } catch (failure) {
      clearTimeout(timer);
      timer = undefined;
      queued = normalize(queued + degrees);
      error = failure;
    } finally {
      flight = false;
    }
    if (!error && queued && !timer) void flush();
    else finish();
  }
  return {
    rotate(degrees) {
      queued = normalize(queued + degrees);
      preview = normalize(preview + degrees);
      error = null;
      clearTimeout(timer);
      timer = queued ? setTimeout(flush, delay) : undefined;
      if (!flight && !queued) finish();
      else notify();
    },
    retry: flush,
    dispose() {
      disposed = true;
      clearTimeout(timer);
      timer = undefined;
      if (!error) void flush();
    },
  };
}
