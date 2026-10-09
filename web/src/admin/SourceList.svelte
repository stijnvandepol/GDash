<script lang="ts">
  import { ask } from './confirm.svelte';
  import { ApiError, type SourceStatus } from './api';
  import { describeStatus } from './status';
  import { CALENDAR_COLORS } from '../colors';

  interface Item {
    id: number;
    name: string;
    /** Shown after the name: the address of a feed, the host of a calendar. */
    detail: string;
    /** A problem with this one item. */
    warning?: string;
    /** The item's colour, for calendars; see colors.ts. */
    color?: string;
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
    oncolor,
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
    /** Changes an item's colour; without it no colour is shown. */
    oncolor?: (id: number, color: string) => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  async function recolor(item: Item, color: string) {
    error = '';
    try {
      await oncolor?.(item.id, color);
    } catch (err) {
      error = onerror(err);
    }
  }

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
    const message = 'It is no longer fetched or shown on the TVs.';
    if (!(await ask({ title: `Remove "${item.name}"?`, message, action: 'Remove' }))) return;
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
          {#if oncolor && item.color}
            <span class="swatch" style:background={CALENDAR_COLORS[item.color]}></span>
            <select
              aria-label="Colour of {item.name}"
              value={item.color}
              onchange={(e) => recolor(item, (e.target as HTMLSelectElement).value)}
            >
              {#each Object.keys(CALENDAR_COLORS) as c (c)}
                <option value={c}>{c}</option>
              {/each}
            </select>
          {/if}
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
        <!-- Not type="password": browsers offer to save those, which would
             put the secret calendar address in a password manager. -->
        <input
          bind:value={url}
          type={secret ? 'text' : 'url'}
          class:masked={secret}
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
  .swatch {
    flex: none;
    width: 14px;
    height: 14px;
    border-radius: 50%;
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
