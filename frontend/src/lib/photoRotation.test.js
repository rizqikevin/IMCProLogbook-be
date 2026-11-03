import { afterEach, expect, test, vi } from "vitest";
import { createPhotoRotation } from "./photoRotation";

afterEach(() => vi.useRealTimers());
function setup(save = vi.fn().mockResolvedValue({ id: 1, sha256: "new" })) {
  vi.useFakeTimers();
  const onChange = vi.fn();
  const onSaved = vi.fn();
  const queue = createPhotoRotation({ photo: { id: 1, sha256: "old" }, save, onChange, onSaved });
  return { queue, save, onChange, onSaved };
}
test("burst previews immediately and sends one net rotation after last click", async () => {
  const { queue, save, onChange } = setup();
  queue.rotate(90);
  expect(onChange).toHaveBeenLastCalledWith({ preview: 90, busy: true, error: null });
  await vi.advanceTimersByTimeAsync(400);
  queue.rotate(90);
  await vi.advanceTimersByTimeAsync(499);
  expect(save).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(save).toHaveBeenCalledExactlyOnceWith({ id: 1, sha256: "old" }, 180);
});
test("full turns and opposite clicks send nothing", async () => {
  const { queue, save, onChange } = setup();
  for (let n = 0; n < 4; n++) queue.rotate(90);
  queue.rotate(-90); queue.rotate(90);
  await vi.advanceTimersByTimeAsync(1000);
  expect(save).not.toHaveBeenCalled();
  expect(onChange).toHaveBeenLastCalledWith({ preview: 0, busy: false, error: null });
});
test("clicks during save serialize against returned hash", async () => {
  let release;
  const save = vi.fn().mockImplementationOnce(() => new Promise((resolve) => { release = resolve; })).mockResolvedValue({ id: 1, sha256: "final" });
  const { queue, onSaved } = setup(save);
  queue.rotate(90);
  await vi.advanceTimersByTimeAsync(500);
  queue.rotate(-90);
  await vi.advanceTimersByTimeAsync(500);
  expect(save).toHaveBeenCalledTimes(1);
  release({ id: 1, sha256: "middle" });
  await vi.advanceTimersByTimeAsync(0);
  expect(save).toHaveBeenNthCalledWith(2, { id: 1, sha256: "middle" }, -90);
  expect(onSaved).toHaveBeenCalledExactlyOnceWith({ id: 1, sha256: "final" });
});
test("failure retains preview and close lock; retry uses unchanged baseline", async () => {
  const failure = new Error("failed");
  const save = vi.fn().mockRejectedValueOnce(failure).mockResolvedValue({ id: 1, sha256: "new" });
  const { queue, onChange } = setup(save);
  queue.rotate(90);
  await vi.advanceTimersByTimeAsync(500);
  expect(onChange).toHaveBeenLastCalledWith({ preview: 90, busy: true, error: failure });
  await queue.retry();
  expect(save).toHaveBeenNthCalledWith(2, { id: 1, sha256: "old" }, 90);
  expect(onChange).toHaveBeenLastCalledWith({ preview: 0, busy: false, error: null });
});
test("unmount flushes queued change without notifying unmounted view", async () => {
  const { queue, save, onChange, onSaved } = setup();
  queue.rotate(90);
  onChange.mockClear();
  queue.dispose();
  await vi.advanceTimersByTimeAsync(1000);
  expect(save).toHaveBeenCalledTimes(1);
  expect(onSaved).toHaveBeenCalledWith({ id: 1, sha256: "new" });
  expect(onChange).not.toHaveBeenCalled();
});
test("save completion waits for newer click debounce deadline", async () => {
  let release;
  const save = vi.fn().mockImplementationOnce(() => new Promise((resolve) => { release = resolve; })).mockResolvedValue({ id: 1, sha256: "final" });
  const { queue } = setup(save);
  queue.rotate(90);
  await vi.advanceTimersByTimeAsync(500);
  queue.rotate(90);
  release({ id: 1, sha256: "middle" });
  await vi.advanceTimersByTimeAsync(499);
  expect(save).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(1);
  expect(save).toHaveBeenNthCalledWith(2, { id: 1, sha256: "middle" }, 90);
});
test("failure cancels automatic queued retry until explicit retry", async () => {
  let reject;
  const failure = new Error("failed");
  const save = vi.fn().mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; })).mockResolvedValue({ id: 1, sha256: "final" });
  const { queue, onChange } = setup(save);
  queue.rotate(90);
  await vi.advanceTimersByTimeAsync(500);
  queue.rotate(90);
  reject(failure);
  await vi.advanceTimersByTimeAsync(1000);
  expect(save).toHaveBeenCalledTimes(1);
  expect(onChange).toHaveBeenLastCalledWith({ preview: 180, busy: true, error: failure });
  await queue.retry();
  expect(save).toHaveBeenNthCalledWith(2, { id: 1, sha256: "old" }, 180);
});
