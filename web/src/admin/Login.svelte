<script lang="ts">
  import { api, ApiError, type User } from './api';
  import Logo from './Logo.svelte';

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

<div class="auth-card">
  <div class="auth-brand">
    <Logo size={56} />
    <span class="name">GDash</span>
    <p>Your home dashboard for the TV.</p>
  </div>

  <form class="card auth-form" onsubmit={submit}>
    <h1>Log in</h1>
    <label>
      Username
      <input bind:value={username} autocomplete="username" required />
    </label>
    <label>
      Password
      <input type="password" bind:value={password} autocomplete="current-password" required />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button type="submit" class:busy aria-busy={busy} disabled={busy}>{busy ? 'Working…' : 'Log in'}</button>
  </form>
</div>
