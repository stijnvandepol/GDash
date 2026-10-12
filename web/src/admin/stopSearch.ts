/** Wait this long after the last keystroke before asking the server. */
export const SEARCH_DELAY_MS = 300;

/** The server refuses shorter queries (see internal/api/departures.go). */
export const MIN_QUERY_LENGTH = 2;

export interface SearchHandlers<T> {
  /** The query is too short to search; clear what is shown. */
  onIdle: () => void;
  /** A search is waiting or running. */
  onBusy: () => void;
  onResult: (result: T) => void;
  onError: (err: unknown) => void;
}

/**
 * Search as you type without a request per keystroke, and without an old
 * answer replacing a newer one: a slow answer for "ut" must not appear after
 * the answer for "utrecht". Only the last query's result or error is passed
 * on.
 */
export function typeAheadSearch<T>(search: (query: string) => Promise<T>, handlers: SearchHandlers<T>) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let latest = 0;

  async function run(query: string, id: number) {
    try {
      const result = await search(query);
      if (id === latest) handlers.onResult(result);
    } catch (err) {
      if (id === latest) handlers.onError(err);
    }
  }

  return {
    /** Call with the whole text of the field after every edit. */
    input(text: string) {
      clearTimeout(timer);
      const query = text.trim();
      const id = ++latest;
      if (query.length < MIN_QUERY_LENGTH) {
        handlers.onIdle();
        return;
      }
      handlers.onBusy();
      timer = setTimeout(() => void run(query, id), SEARCH_DELAY_MS);
    },
    /** Drops a waiting search and ignores one that is running. */
    cancel() {
      clearTimeout(timer);
      latest++;
    },
  };
}
