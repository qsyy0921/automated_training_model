export function addTrackToDeleteQueue(queue: string[], trackKey: string): string[] {
  if (!trackKey || queue.includes(trackKey)) return queue;
  return [...queue, trackKey];
}

export function removeTrackFromDeleteQueue(queue: string[], trackKey: string): string[] {
  return queue.filter((item) => item !== trackKey);
}

