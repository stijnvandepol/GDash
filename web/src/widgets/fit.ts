/**
 * Hides the list items of a widget that do not fit entirely, so a list never
 * ends in a half-cut line on the TV. A group marked data-fit-group, such as a
 * calendar day, is hidden when none of its items is visible. It measures
 * again when the widget's size or content changes.
 */
export function fitItems(node: HTMLElement) {
  const fit = () => {
    const items = [...node.querySelectorAll<HTMLElement>('li')];
    const groups = [...node.querySelectorAll<HTMLElement>('[data-fit-group]')];
    for (const el of [...items, ...groups]) el.hidden = false;
    const limit = node.getBoundingClientRect().bottom;
    // Once one item overflows, every later one is hidden too, so a short
    // item further down never shows up after a gap.
    let full = false;
    for (const li of items) {
      full ||= li.getBoundingClientRect().bottom > limit + 0.5;
      li.hidden = full;
    }
    for (const group of groups) group.hidden = group.querySelector('li:not([hidden])') === null;
  };

  const resize = new ResizeObserver(fit);
  resize.observe(node);
  // Only content changes; toggling hidden is an attribute change and does
  // not trigger another round.
  const content = new MutationObserver(fit);
  content.observe(node, { childList: true, subtree: true, characterData: true });
  fit();

  return {
    destroy() {
      resize.disconnect();
      content.disconnect();
    },
  };
}

const SHRINK_STEP_PX = 2;

/**
 * Lowers the font size of a text block until it fits its box, down to
 * minPx, so a long note in a small tile stays whole. Below minPx the text
 * would be unreadable across a room; then it is clipped instead.
 */
export function shrinkToFit(node: HTMLElement, minPx: number) {
  const fit = () => {
    node.style.fontSize = '';
    let size = parseFloat(getComputedStyle(node).fontSize);
    while (node.scrollHeight > node.clientHeight + 1 && size > minPx) {
      size = Math.max(minPx, size - SHRINK_STEP_PX);
      node.style.fontSize = `${size}px`;
    }
  };
  const resize = new ResizeObserver(fit);
  resize.observe(node);
  const content = new MutationObserver(fit);
  content.observe(node, { childList: true, subtree: true, characterData: true });
  fit();
  return {
    destroy() {
      resize.disconnect();
      content.disconnect();
    },
  };
}
