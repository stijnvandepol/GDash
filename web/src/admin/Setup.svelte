<script lang="ts">
  import { api, ApiError, type User } from './api';

  let { onDone }: { onDone: (user: User) => void } = $props();

  let setupToken = $state('');
  let username = $state('');
  let password = $state('');
  let error = $state<ApiError | null>(null);
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = null;
    try {
      onDone(await api.setup(setupToken.trim(), username, password));
    } catch (err) {
      error = err instanceof ApiError ? err : new ApiError(0, 'network', 'The server cannot be reached.');
    } finally {
      busy = false;
    }
  }
</script>

<form class="card narrow" onsubmit={submit}>
  <h1>Set up Daycast</h1>
  <p class="muted">
    Create the administrator account. The setup token is printed in the Daycast log, for example in Container Manager on
    your NAS or in the terminal window. That way only you can set up this server.
  </p>

  <label>
    Setup token
    <input bind:value={setupToken} autocomplete="off" spellcheck="false" required />
  </label>
  <label>
    Username
    <input bind:value={username} autocomplete="username" required />
    {#if error?.fieldMessage('username')}<span class="field-error">{error.fieldMessage('username')}</span>{/if}
  </label>
  <label>
    Password
    <input type="password" bind:value={password} autocomplete="new-password" required minlength="10" />
    {#if error?.fieldMessage('password')}<span class="field-error">{error.fieldMessage('password')}</span>{/if}
  </label>

  {#if error && error.fields.length === 0}<p class="error">{error.message}</p>{/if}
  <button type="submit" disabled={busy}>{busy ? 'Working…' : 'Create account'}</button>
</form>
