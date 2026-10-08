<script lang="ts">
  import { api, ApiError, type User } from './api';

  let { onDone }: { onDone: (user: User) => void } = $props();

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      onDone(await api.login(username, password));
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'The server cannot be reached.';
    } finally {
      busy = false;
    }
  }
</script>

<form class="card narrow" onsubmit={submit}>
  <h1>Log in</h1>
  <label>
    Username
    <input bind:value={username} autocomplete="username" required />
  </label>
  <label>
    Password
    <input type="password" bind:value={password} autocomplete="current-password" required />
  </label>
  {#if error}<p class="error">{error}</p>{/if}
  <button type="submit" disabled={busy}>{busy ? 'Working…' : 'Log in'}</button>
</form>
