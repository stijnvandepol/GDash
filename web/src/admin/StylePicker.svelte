<script lang="ts">
  import StyleSample from './StyleSample.svelte';
  import { FONTS, GROUPS, THEMES, nightChoices, themeName, type FontId } from './styles';

  // The dashboard's style: a theme from a gallery of samples, and a typeface
  // that is the theme's own or one of the others, and the theme to switch to
  // after sunset. Choices apply at once; the editor saves them with the
  // layout.
  let {
    theme = $bindable(),
    font = $bindable(),
    nightTheme = $bindable(),
    themes,
    fonts,
    onchange,
    onclose,
  }: {
    theme: string;
    font: string;
    /** The theme after sunset; empty keeps the theme. */
    nightTheme: string;
    /** The themes and fonts the server accepts, in its order. */
    themes: string[];
    fonts: string[];
    onchange: () => void;
    onclose: () => void;
  } = $props();

  let dialog: HTMLDialogElement;

  // Opened with the current theme in focus, so arrow keys move from there.
  $effect(() => {
    dialog.showModal();
    dialog.querySelector<HTMLInputElement>('input[name="theme"]:checked')?.focus();
  });

  function pickTheme(id: string) {
    theme = id;
    onchange();
  }

  function pickFont(id: string) {
    font = id;
    onchange();
  }

  function pickNightTheme(event: Event) {
    nightTheme = (event.currentTarget as HTMLSelectElement).value;
    onchange();
  }

  // The server's themes in its order, sorted into the picker's sections; a
  // theme this editor has no notes for goes with the essentials.
  const sections = $derived(
    GROUPS.map((g) => ({ ...g, themes: themes.filter((id) => (THEMES[id]?.group ?? 'essentials') === g.id) })),
  );

  const nightOptions = $derived(nightChoices(theme, themes, nightTheme));

  const ownFont = $derived(FONTS[(THEMES[theme]?.font ?? 'geist') as FontId]);
</script>

<dialog bind:this={dialog} aria-labelledby="style-title" onclose={onclose}>
  <header class="head">
    <h2 id="style-title">Style</h2>
    <button type="button" onclick={() => dialog.close()}>Done</button>
  </header>

  <fieldset>
    <legend>Theme</legend>
    {#each sections as section (section.id)}
    <div class="section-head">
      <h3>{section.name}</h3>
      <p class="muted small">{section.note}</p>
    </div>
    <div class="themes">
      {#each section.themes as id (id)}
        <label class="choice" class:chosen={theme === id}>
          <input class="sr-only" type="radio" name="theme" value={id} checked={theme === id} onchange={() => pickTheme(id)} />
          <StyleSample theme={id} {font} />
          <span class="name">{THEMES[id]?.name ?? id}</span>
          <span class="note muted small">{THEMES[id]?.note ?? ''}</span>
        </label>
      {/each}
    </div>
    {/each}
  </fieldset>

  <fieldset>
    <legend>Typeface</legend>
    <div class="fonts screen" data-theme={theme}>
      <label class="face" class:chosen={font === ''}>
        <input class="sr-only" type="radio" name="font" value="" checked={font === ''} onchange={() => pickFont('')} />
        <span class="specimen" aria-hidden="true" style:font-family="var(--face-{THEMES[theme]?.font ?? 'geist'})">Aa 21:45</span>
        <span class="small">The theme’s own, {ownFont}</span>
      </label>
      {#each fonts as id (id)}
        <label class="face" class:chosen={font === id}>
          <input class="sr-only" type="radio" name="font" value={id} checked={font === id} onchange={() => pickFont(id)} />
          <span class="specimen" aria-hidden="true" style:font-family="var(--face-{id})">Aa 21:45</span>
          <span class="small">{FONTS[id as FontId] ?? id}</span>
        </label>
      {/each}
    </div>
  </fieldset>

  <fieldset class="night">
    <legend>After sunset</legend>
    <div class="night-pick">
      <label for="night-theme">Style between sunset and sunrise</label>
      <select id="night-theme" value={nightTheme} aria-describedby="night-hint" onchange={pickNightTheme}>
        <option value="">Keep this style</option>
        {#each nightOptions as id (id)}
          <option value={id}>{themeName(id)}</option>
        {/each}
      </select>
    </div>
    <p id="night-hint" class="muted small">
      Follows sunset and sunrise at the weather location, with the same typeface. Without a weather location under Integrations the style never changes.
    </p>
  </fieldset>
</dialog>

<style>
  dialog {
    width: min(100% - 2 * var(--gutter), 1080px);
    max-height: min(100dvh - 2 * var(--s-5), 960px);
    padding: 0 var(--s-5) var(--s-5);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--raised);
    color: var(--text);
    box-shadow: var(--shadow-md);
  }
  dialog::backdrop {
    background: var(--scrim);
  }
  .head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    padding: var(--s-4) 0;
    background: var(--raised);
    border-bottom: 1px solid var(--border);
  }
  fieldset {
    margin: 0;
    padding: var(--s-5) 0 0;
    border: 0;
    min-width: 0;
  }
  /* Floated, so it sits inside the fieldset instead of on its border; the
     grids below clear it. */
  legend {
    float: left;
    width: 100%;
    margin-bottom: var(--s-3);
    padding: 0;
    font-weight: 600;
  }
  .section-head,
  .fonts,
  .night-pick {
    clear: both;
  }
  .section-head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-1) var(--s-3);
    margin: var(--s-5) 0 var(--s-3);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  legend + .section-head {
    margin-top: 0;
  }
  .section-head h3 {
    font-size: var(--fs-md);
  }
  .themes {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 200px), 1fr));
    gap: var(--s-5) var(--s-4);
  }
  /* A choice is a card you press; the chosen one is ringed in the signal
     colour, and the ring follows the keyboard too. */
  .choice,
  .face {
    display: grid;
    align-content: start;
    gap: 6px;
    padding: 6px;
    border-radius: var(--r-lg);
    outline: 2px solid transparent;
    outline-offset: 0;
    cursor: pointer;
    font-weight: 400;
  }
  .choice:hover,
  .face:hover {
    outline-color: var(--border-strong);
  }
  .choice.chosen,
  .face.chosen {
    outline-color: var(--focus);
  }
  .choice:has(:focus-visible),
  .face:has(:focus-visible) {
    outline: 3px solid var(--focus);
    outline-offset: 2px;
  }
  /* The card shows the focus; the hidden radio button itself does not. */
  .choice input,
  .face input {
    outline: none;
  }
  .name {
    font-weight: 600;
    color: var(--text);
  }
  .note {
    line-height: 1.35;
  }
  /* A phone: two small screens a row, without their notes. */
  @container page (max-width: 520px) {
    .themes {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--s-3);
    }
    .note {
      display: none;
    }
  }
  /* The typefaces, each shown in itself. The theme's palette is not used
     here, only its list of faces. */
  .fonts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 150px), 1fr));
    gap: var(--s-2);
  }
  .face {
    padding: var(--s-3);
    border: 1px solid var(--border);
    color: var(--text);
  }
  .night-pick {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
  }
  .night-pick label {
    font-weight: 450;
  }
  .night-pick select {
    width: auto;
    min-width: 200px;
  }
  .night p {
    margin-top: var(--s-2);
  }
  .specimen {
    font-size: 1.5rem;
    line-height: 1.1;
  }
</style>
