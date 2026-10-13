<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type SourceStatus, type Synology } from './api';
  import { describeStatus } from './status';

  let {
    album,
    status,
    onchanged,
    onerror,
  }: {
    album: Synology | null;
    /** Present once a Photos widget shows the album. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let link = $state('');
  let password = $state('');
  let acceptCertificate = $state(false);
  let fieldErrors = $state<{ link?: string; password?: string; acceptCertificate?: string }>({});
  let error = $state('');
  let busy = $state(false);
  // Shows the form over an existing album, to change the link or password.
  let editing = $state(false);

  async function connect(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      const connected = await api.connectSynology(link.trim(), password, acceptCertificate);
      toast.success(`The album is connected: GDash can show ${connected.photos} ${connected.photos === 1 ? 'photo' : 'photos'}.`);
      link = '';
      password = '';
      acceptCertificate = false;
      editing = false;
      await onchanged();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = {
          link: err.fieldMessage('link'),
          password: err.fieldMessage('password'),
          acceptCertificate: err.fieldMessage('acceptCertificate'),
        };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function disconnect() {
    const message = 'Photos widgets set to Synology Photos then show that it is not set up. The link and password are deleted.';
    if (!(await ask({ title: 'Disconnect the Synology Photos album?', message, action: 'Disconnect' }))) return;
    error = '';
    try {
      await api.disconnectSynology();
      toast.success('The Synology Photos album is disconnected.');
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Synology Photos</h2>
  <p class="muted">
    Show an album of Synology Photos on your NAS in Photos widgets: choose Synology Photos album in the widget's
    settings. In Synology Photos, open the album, choose Share, turn on the share link and copy it; a password on the
    link is fine. GDash needs no account on the NAS, fetches the photos itself, so the TVs never see the NAS or the
    link, and stores the link and password encrypted. A random part of the album changes every 30 minutes; videos are
    left out.
  </p>
  {#if album && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          <strong>Shared album on {album.host}</strong>
          <span class="muted small">{album.passwordSet ? 'With a password' : 'Without a password'}</span>
          {#if album.certificate}
            <span class="muted small fingerprint">Accepted certificate: {album.certificate}</span>
          {/if}
          {#if album.unreadable}
            <span class="field-error">This album cannot be read on this server. Connect it again.</span>
          {/if}
        </div>
        <button class="link" onclick={() => (editing = true)}>Change</button>
        <button class="link danger" onclick={disconnect}>Disconnect</button>
      </li>
    </ul>
    {#if status}
      <p class="small" class:error={status.failing}>{describeStatus(status)}</p>
    {:else}
      <p class="small muted">Set a Photos widget to Synology Photos album to show it.</p>
    {/if}
  {:else}
    <form onsubmit={connect}>
      <label>
        Share link
        <input
          bind:value={link}
          type="url"
          placeholder="https://192.168.1.2:5001/mo/sharing/AbCd12"
          autocomplete="off"
          spellcheck="false"
          required
        />
        <span class="small muted">
          Use the NAS's address on your home network. Your TVs never get this link.
        </span>
        {#if fieldErrors.link}<span class="field-error">{fieldErrors.link}</span>{/if}
      </label>
      <label>
        <span>Password of the link <span class="muted">(if it has one)</span></span>
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input
          bind:value={password}
          class="masked"
          autocomplete="off"
          spellcheck="false"
          placeholder={album?.passwordSet ? 'Stored; leave empty to keep it' : ''}
        />
        {#if fieldErrors.password}<span class="field-error">{fieldErrors.password}</span>{/if}
      </label>
      <label class="check">
        <input type="checkbox" bind:checked={acceptCertificate} />
        <span>
          Accept this NAS's certificate
          <span class="small muted">
            For an https link to a NAS with its own, self-signed certificate. GDash remembers the certificate the NAS
            shows now and accepts only that one; after the NAS gets a new certificate, connect again. On your home
            network, the http address on port 5000 works without this.
          </span>
        </span>
      </label>
      {#if fieldErrors.acceptCertificate}<span class="field-error">{fieldErrors.acceptCertificate}</span>{/if}
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Connecting…' : 'Connect'}</button>
        {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
      </div>
    </form>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .masked {
    -webkit-text-security: disc;
  }
  .fingerprint {
    display: block;
    font-family: var(--mono);
    overflow-wrap: anywhere;
  }
  .check {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    font-weight: 400;
    max-width: 40rem;
  }
  .check span {
    display: grid;
    gap: 2px;
  }
</style>
