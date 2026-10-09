<script lang="ts">
  import { api, ApiError, type User } from './api';
  import Logo from './Logo.svelte';

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

<div class="auth-card">
  <div class="auth-brand">
    <Logo size={56} />
    <span class="name">Welcome to GDash</span>
    <p>Create the administrator account to get started.</p>
  </div>

  <form class="card auth-form" onsubmit={submit}>
    <h1>Set up this server</h1>

    <label>
      Setup token
      <input bind:value={setupToken} autocomplete="off" spellcheck="false" required />
    </label>
    <details class="help" open>
      <summary>Where do I find the setup token?</summary>
      <p>
        GDash prints it in its log when it starts for the first time. Only someone with access to the server can read
        it, so only you can set up this server.
      </p>
      <ul>
        <li>Docker: run <code>docker compose logs gdash</code></li>
        <li>Synology: open the project in Container Manager and read its log</li>
        <li>Linux service: run <code>journalctl -u gdash</code></li>
        <li>Started in a terminal: it is in that window</li>
      </ul>
    </details>

    <label>
      Username
      <input bind:value={username} autocomplete="username" required aria-invalid={!!error?.fieldMessage('username')} />
      {#if error?.fieldMessage('username')}<span class="field-error">{error.fieldMessage('username')}</span>{/if}
    </label>
    <label>
      Password
      <input
        type="password"
        bind:value={password}
        autocomplete="new-password"
        required
        minlength="12"
        aria-invalid={!!error?.fieldMessage('password')}
      />
      <span class="muted small">At least 12 characters; a few words in a row are easy to remember.</span>
      {#if error?.fieldMessage('password')}<span class="field-error">{error.fieldMessage('password')}</span>{/if}
    </label>

    {#if error && error.fields.length === 0}<p class="error" role="alert">{error.message}</p>{/if}
    <button type="submit" class:busy aria-busy={busy} disabled={busy}>{busy ? 'Working…' : 'Create account'}</button>
  </form>
</div>
