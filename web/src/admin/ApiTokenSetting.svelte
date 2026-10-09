<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { onMount } from 'svelte';
  import { api, ApiError, type ApiToken, type NewApiToken } from './api';
  import { describeLastUse } from './apiTokens';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  // The server allows this many (internal/store.MaxAPITokens).
  const MAX_TOKENS = 10;

  let tokens = $state<ApiToken[]>([]);
  let name = $state('');
  let nameError = $state('');
  let error = $state('');
  let busy = $state(false);
  // The secret of the token that was just made. It is not on the server
  // anymore, so it is shown until this card is left or another is made.
  let fresh = $state<NewApiToken | null>(null);
  const made = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });

  onMount(() => void load());

  async function load() {
    try {
      tokens = await api.apiTokens();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function create(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = nameError = '';
    try {
      fresh = await api.createApiToken(name.trim());
      name = '';
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        nameError = err.fieldMessage('name') ?? err.message;
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function revoke(token: ApiToken) {
    const message = `Automations that use "${token.name}" stop working at once.`;
    if (!(await ask({ title: 'Revoke this token?', message, action: 'Revoke' }))) return;
    error = '';
    try {
      await api.revokeApiToken(token.id);
      toast.success(`Revoked ${token.name}.`);
      if (fresh?.id === token.id) fresh = null;
      await load();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>API tokens</h2>
  <p class="muted">
    Let an automation, such as Home Assistant, put announcements on the TVs. Make a token per automation, so you can
    revoke one without touching the others. A token can only send announcements, nothing else.
  </p>
  {#if fresh}
    <div class="fresh">
      <p class="small"><strong>Copy the token for "{fresh.name}" now.</strong> It is not shown again.</p>
      <input readonly value={fresh.token} aria-label="New API token" onfocus={(e) => (e.target as HTMLInputElement).select()} />
      <button class="link" onclick={() => (fresh = null)}>I have saved it</button>
    </div>
  {/if}
  {#if tokens.length > 0}
    <ul class="list">
      {#each tokens as token (token.id)}
        <li>
          <div class="grow">
            <strong>{token.name}</strong>
            <span class="muted small">Made {made.format(new Date(token.createdAt))} · {describeLastUse(token)}</span>
          </div>
          <button class="link danger" onclick={() => revoke(token)}>Revoke</button>
        </li>
      {/each}
    </ul>
  {/if}
  {#if tokens.length < MAX_TOKENS}
    <form onsubmit={create}>
      <label>
        Name
        <input bind:value={name} placeholder="Home Assistant" maxlength="40" autocomplete="off" required />
        {#if nameError}<span class="field-error">{nameError}</span>{/if}
      </label>
      <button type="submit" disabled={busy}>Make token</button>
    </form>
  {:else}
    <p class="muted">The limit of {MAX_TOKENS} tokens is reached. Revoke one you no longer use.</p>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .fresh {
    display: grid;
    gap: 8px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg);
  }
  .fresh input {
    font-family: ui-monospace, monospace;
    font-size: 0.9rem;
  }
  .list {
    overflow-wrap: anywhere;
  }
</style>
