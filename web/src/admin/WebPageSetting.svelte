<script lang="ts">
  import { onMount } from 'svelte';
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, type WebPage } from './api';
  import { hostOf } from './customSources';
  import { MAX_WEB_PAGES, describeCheck, describeRefresh } from './webPages';
  import WebPageForm from './WebPageForm.svelte';

  let {
    onchanged,
    onerror,
  }: {
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let pages = $state<WebPage[]>([]);
  let error = $state('');
  // 'new', the id of the page being edited, or null.
  let editing = $state<'new' | number | null>(null);
  let checking = $state<number | null>(null);
  let result = $state<{ id: number; ok: boolean; message: string } | null>(null);

  onMount(() => void load());

  async function load() {
    try {
      pages = await api.webPages();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function saved(page: WebPage) {
    toast.success(`${page.name} is saved.`);
    editing = null;
    if (result?.id === page.id) result = null;
    await load();
    await onchanged();
  }

  async function remove(page: WebPage) {
    const message = 'Web page widgets that show it then say that the page is not available.';
    if (!(await ask({ title: `Remove ${page.name}?`, message, action: 'Remove' }))) return;
    error = '';
    try {
      await api.deleteWebPage(page.id);
      if (result?.id === page.id) result = null;
      await load();
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function check(page: WebPage) {
    error = '';
    checking = page.id;
    try {
      result = { id: page.id, ...describeCheck(await api.checkWebPage(page.id)) };
    } catch (err) {
      result = null;
      error = onerror(err);
    } finally {
      checking = null;
    }
  }
</script>

<section class="card">
  <h2>Web pages</h2>
  <p class="muted">
    Show a page in a tile, such as a Grafana panel, a Home Assistant dashboard in kiosk mode, a weather radar or a
    status page on your network. Add it here, then add a Web page widget to a dashboard and choose it by name. The TV
    shows the page without interaction. Many sites forbid being shown in a frame: use Check to find out.
  </p>
  {#if pages.length > 0}
    <ul class="list">
      {#each pages as page (page.id)}
        <li>
          <div class="grow">
            <strong>{page.name}</strong>
            <span class="muted small">{hostOf(page.url)} · {describeRefresh(page.refreshMinutes)} · {page.zoom}%{page.allowScripts ? '' : ' · No scripts'}</span>
          </div>
          <button class="link" disabled={checking !== null} onclick={() => check(page)}>{checking === page.id ? 'Checking…' : 'Check'}</button>
          <button class="link" onclick={() => (editing = page.id)}>Edit</button>
          <button class="link danger" onclick={() => remove(page)}>Remove</button>
        </li>
        {#if editing === page.id}
          <li class="form-row">
            <WebPageForm {page} onsaved={saved} oncancel={() => (editing = null)} {onerror} />
          </li>
        {/if}
        {#if result?.id === page.id}
          <li class="form-row">
            <p class="small" class:field-error={!result.ok} class:ok={result.ok} role="status">{result.message}</p>
          </li>
        {/if}
      {/each}
    </ul>
  {/if}
  {#if editing === 'new'}
    <WebPageForm onsaved={saved} oncancel={() => (editing = null)} {onerror} />
  {:else if pages.length < MAX_WEB_PAGES}
    <div><button class="secondary" onclick={() => (editing = 'new')}>Add a web page</button></div>
  {:else}
    <p class="small muted">The limit of {MAX_WEB_PAGES} web pages is reached.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .form-row {
    display: block;
  }
  .form-row p {
    margin: 0;
  }
  .ok {
    color: var(--success);
  }
</style>
