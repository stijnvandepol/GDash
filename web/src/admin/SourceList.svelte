<script lang="ts">
  import { ApiError, type SourceStatus } from './api';
  import { describeStatus } from './status';

  interface Item {
    id: number;
    name: string;
    /** Shown after the name: the address of a feed, the host of a calendar. */
    detail: string;
    /** A problem with this one item. */
    warning?: string;
  }

  let {
    title,
    intro,
    items,
    status,
    max,
    secret = false,
    namePlaceholder,
    urlPlaceholder,
    onadd,
    onremove,
    onerror,
  }: {
    title: string;
    intro: string;
    items: Item[];
    /** The status of the data source these items feed, if configured. */
    status: SourceStatus | undefined;
    max: number;
    /** Hide the address while typing, for calendar addresses. */
    secret?: boolean;
    namePlaceholder: string;
    urlPlaceholder: string;
    onadd: (name: string, url: string) => Promise<void>;
    onremove: (id: number) => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let name = $state('');
  let url = $state('');
  let fieldErrors = $state<{ name?: string; url?: string }>({});
  let error = $state('');
  let busy = $state(false);

  async function add(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await onadd(name.trim(), url.trim());
      name = '';
      url = '';
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { name: err.fieldMessage('name'), url: err.fieldMessage('url') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function remove(item: Item) {
    if (!confirm(`Remove "${item.name}"?`)) return;
    error = '';
    try {
      await onremove(item.id);
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>{title}</h2>
  <p class="muted">{intro}</p>
  {#if items.length > 0}
    <p class="small" class:error={status?.failing}>{describeStatus(status)}</p>
    <ul class="list">
      {#each items as item (item.id)}
        <li>
          <div class="grow">
            <strong>{item.name}</strong>
            <span class="muted address">{item.detail}</span>
            {#if item.warning}<span class="field-error">{item.warning}</span>{/if}
          </div>
          <button class="link danger" onclick={() => remove(item)}>Remove</button>
        </li>
      {/each}
    </ul>
  {/if}
  {#if items.length < max}
    <form onsubmit={add}>
      <label>
        Name
        <input bind:value={name} placeholder={namePlaceholder} required maxlength="60" />
        {#if fieldErrors.name}<span class="field-error">{fieldErrors.name}</span>{/if}
      </label>
      <label>
        Address
        <input
          bind:value={url}
          type={secret ? 'password' : 'url'}
          placeholder={urlPlaceholder}
          autocomplete="off"
          spellcheck="false"
          required
        />
        {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
      </label>
      <button type="submit" disabled={busy}>Add</button>
    </form>
  {:else}
    <p class="muted">You reached the limit of {max}. Remove one to add another.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .address {
    display: block;
    overflow-wrap: anywhere;
  }
</style>
