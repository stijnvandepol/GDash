<script lang="ts">
  import { api, ApiError, type WebPage, type WebPageInput } from './api';
  import { emptyWebPage, toWebPageInput } from './webPages';

  let {
    page,
    onsaved,
    oncancel,
    onerror,
  }: {
    /** The page to change, or undefined for a new one. */
    page?: WebPage;
    onsaved: (saved: WebPage) => Promise<void>;
    oncancel: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The form starts from the page once; a reload of the list must not
  // overwrite what is being typed.
  // svelte-ignore state_referenced_locally
  let form = $state<WebPageInput>(page ? toWebPageInput(page) : emptyWebPage());
  let fieldErrors = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    fieldErrors = {};
    busy = true;
    try {
      const input = { ...form, refreshMinutes: Number(form.refreshMinutes) || 0, zoom: Number(form.zoom) || 100 };
      const saved = page ? await api.saveWebPage(page.id, input) : await api.addWebPage(input);
      await onsaved(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={save} class="source-form">
  <label>
    Name
    <input bind:value={form.name} maxlength="60" required placeholder="Energy in Grafana" />
    {#if fieldErrors.name}<span class="field-error">{fieldErrors.name}</span>{/if}
  </label>
  <label>
    Address
    <input bind:value={form.url} type="url" required autocomplete="off" spellcheck="false" placeholder="http://192.168.1.10:3000/d/energy?orgId=1&kiosk" />
    <span class="small muted">
      A kiosk or share link works best: the TV cannot log in. A token in the address is shown here to every admin, and
      reaches only the screens that show this page.
    </span>
    {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
  </label>
  <div class="row">
    <label>
      Reload every (minutes)
      <input type="number" min="0" max="1440" bind:value={form.refreshMinutes} />
      <span class="small muted">0 never reloads. Grafana and Home Assistant update themselves.</span>
      {#if fieldErrors.refreshMinutes}<span class="field-error">{fieldErrors.refreshMinutes}</span>{/if}
    </label>
    <label>
      Zoom (%)
      <input type="number" min="50" max="200" step="10" bind:value={form.zoom} />
      <span class="small muted">Below 100 fits more of the page in a tile.</span>
      {#if fieldErrors.zoom}<span class="field-error">{fieldErrors.zoom}</span>{/if}
    </label>
  </div>
  <label class="check">
    <input type="checkbox" bind:checked={form.allowScripts} />
    Let the page run scripts
  </label>
  <span class="small muted">
    Most pages need this. The page never gets to open windows, leave the tile or download files. Without scripts it is
    shown as plain, fully sandboxed HTML.
  </span>

  <div class="inline">
    <button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
    <button type="button" class="link" onclick={oncancel}>Cancel</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</form>

<style>
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-4);
    max-width: 40rem;
  }
  .row label {
    flex: 1 1 12rem;
    /* Inputs side by side line up when one hint wraps and the other does not. */
    align-content: start;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
</style>
