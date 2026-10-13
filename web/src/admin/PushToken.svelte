<script lang="ts">
  import { api, ApiError } from './api';

  // Makes an API token for a system that sends data to GDash, and hands it
  // to the examples. Like under Automations, the secret is shown once.
  let {
    name,
    ontoken,
    onmanagetokens,
    onerror,
  }: {
    /** The name the token gets, such as the source's. */
    name: string;
    ontoken: (token: string) => void;
    /** Opens the API tokens under Automations. */
    onmanagetokens: () => void;
    onerror: (err: unknown) => string;
  } = $props();

  // The server allows names of this many characters (internal/apitoken).
  const MAX_TOKEN_NAME = 40;

  let token = $state('');
  let error = $state('');
  let busy = $state(false);

  async function make() {
    busy = true;
    error = '';
    try {
      const made = await api.createApiToken(name.slice(0, MAX_TOKEN_NAME));
      token = made.token;
      ontoken(made.token);
    } catch (err) {
      error = err instanceof ApiError && err.status === 409 ? `${err.message} Revoke one under Automations.` : onerror(err);
    } finally {
      busy = false;
    }
  }
</script>

<div class="token">
  {#if token}
    <p class="small"><strong>Copy the token now.</strong> It is not shown again; the examples below already hold it.</p>
    <input readonly value={token} aria-label="New API token" onfocus={(e) => (e.target as HTMLInputElement).select()} />
  {:else}
    <p class="small muted">
      Your system proves it may send with a GDash API token. Make one for it here, or use one you already have.
    </p>
  {/if}
  <div class="inline">
    {#if !token}
      <button type="button" class="secondary" disabled={busy} onclick={make}>{busy ? 'Making…' : 'Make an API token'}</button>
    {/if}
    <button type="button" class="link" onclick={onmanagetokens}>Manage API tokens</button>
  </div>
  {#if error}<p class="error">{error}</p>{/if}
</div>

<style>
  .token {
    display: grid;
    gap: var(--s-2);
  }
  .token p {
    margin: 0;
  }
  input {
    font-family: var(--mono);
    font-size: var(--fs-sm);
  }
</style>
