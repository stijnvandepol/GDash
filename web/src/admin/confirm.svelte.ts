// Asks the admin to confirm an action in the interface's own dialog,
// instead of the browser's confirm(), which cannot be styled, names the
// site's address in its title and blocks the whole page.
//
// Usage: if (!(await ask({ title: 'Remove display?', message, action: 'Remove' }))) return;

export interface Question {
  title: string;
  message: string;
  /** The label of the button that does it, such as "Remove". */
  action: string;
  /** True for actions that cannot be undone; the button is red. */
  danger?: boolean;
}

interface Pending extends Question {
  answer: (confirmed: boolean) => void;
}

/** The question on screen, read by ConfirmDialog.svelte. */
export const confirmation = $state<{ pending: Pending | null }>({ pending: null });

/** Shows the question and resolves with the admin's answer. A second question replaces the first, which counts as cancelled. */
export function ask(question: Question): Promise<boolean> {
  confirmation.pending?.answer(false);
  return new Promise((resolve) => {
    confirmation.pending = {
      danger: true,
      ...question,
      answer: (confirmed) => {
        confirmation.pending = null;
        resolve(confirmed);
      },
    };
  });
}
