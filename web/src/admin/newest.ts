/**
 * Wraps a loader so only the newest call's result counts. A page that
 * polls and also reloads after every change can get answers out of order;
 * an older answer arriving last would undo what the user just saw happen.
 * A superseded call resolves to null.
 */
export function newestOnly<T>(load: () => Promise<T>): () => Promise<{ value: T } | null> {
  let newest = 0;
  return async () => {
    const call = ++newest;
    const value = await load();
    return call === newest ? { value } : null;
  };
}
