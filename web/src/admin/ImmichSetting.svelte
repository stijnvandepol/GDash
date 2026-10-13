<script lang="ts">
  import { toast } from './toast.svelte';
  import { ask } from './confirm.svelte';
  import { api, ApiError, type Immich, type ImmichAlbum, type SourceStatus } from './api';
  import { describeStatus } from './status';

  let {
    connection,
    status,
    onchanged,
    onerror,
  }: {
    connection: Immich | null;
    /** Present once a Photos widget shows the album. */
    status: SourceStatus | undefined;
    onchanged: () => Promise<void>;
    onerror: (err: unknown) => string;
  } = $props();

  let url = $state('');
  let key = $state('');
  let fieldErrors = $state<{ url?: string; key?: string }>({});
  let error = $state('');
  let busy = $state(false);
  // Shows the form over an existing connection, to change the address or key.
  let editing = $state(false);
  let albums = $state<ImmichAlbum[] | null>(null);

  async function connect(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    fieldErrors = {};
    try {
      await api.connectImmich(url.trim(), key.trim());
      toast.success('Immich is connected.');
      url = '';
      key = '';
      editing = false;
      await onchanged();
      await loadAlbums();
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        fieldErrors = { url: err.fieldMessage('url'), key: err.fieldMessage('key') };
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function loadAlbums() {
    error = '';
    try {
      albums = await api.immichAlbums();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function choose(albumId: string) {
    if (!albumId) return;
    error = '';
    try {
      const chosen = await api.chooseImmichAlbum(albumId);
      toast.success(`Photos widgets set to Immich show ${chosen.albumName}.`);
      albums = null;
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }

  async function disconnect() {
    const message = 'Photos widgets set to Immich then show that it is not set up. The API key is deleted.';
    if (!(await ask({ title: 'Disconnect Immich?', message, action: 'Disconnect' }))) return;
    error = '';
    try {
      await api.disconnectImmich();
      albums = null;
      toast.success('Immich is disconnected.');
      await onchanged();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Immich</h2>
  <p class="muted">
    Show an album of your own Immich server in Photos widgets: choose Immich album in the widget's settings. GDash
    fetches the photos itself, so the TVs never see Immich's address or key; a random part of the album changes
    every 30 minutes, and videos are left out. In Immich, under Account settings, API keys, make a key with at least
    album.read, asset.read and asset.view; GDash stores it encrypted and never shows it again.
  </p>
  {#if connection && !editing}
    <ul class="list">
      <li>
        <div class="grow">
          <strong>{connection.albumName ? `Album: ${connection.albumName}` : 'Connected, no album chosen'}</strong>
          <span class="muted address">{connection.url}</span>
          {#if connection.unreadable}
            <span class="field-error">This connection cannot be read on this server. Connect again.</span>
          {/if}
        </div>
        <button class="link" onclick={loadAlbums}>{connection.albumId ? 'Other album' : 'Choose album'}</button>
        <button class="link" onclick={() => (editing = true)}>Change</button>
        <button class="link danger" onclick={disconnect}>Disconnect</button>
      </li>
    </ul>
    {#if albums}
      {#if albums.length === 0}
        <p class="field-error">This key sees no albums. Make an album in Immich, or a key of the account that has it.</p>
      {:else}
        <label>
          Album
          <select value={connection.albumId} onchange={(e) => choose((e.target as HTMLSelectElement).value)}>
            <option value="">Choose an album</option>
            {#each albums as album (album.id)}
              <option value={album.id}>{album.name} ({album.assetCount})</option>
            {/each}
          </select>
        </label>
      {/if}
    {:else if status}
      <p class="small" class:error={status.failing}>{describeStatus(status)}</p>
    {:else if connection.albumId}
      <p class="small muted">Set a Photos widget to Immich album to show it.</p>
    {/if}
  {:else}
    <form onsubmit={connect}>
      <label>
        Address
        <input bind:value={url} type="url" placeholder="http://192.168.1.10:2283" autocomplete="off" spellcheck="false" required />
        {#if fieldErrors.url}<span class="field-error">{fieldErrors.url}</span>{/if}
      </label>
      <label>
        API key
        <!-- Not type="password", so browsers do not offer to save it. -->
        <input bind:value={key} class="masked" autocomplete="off" spellcheck="false" required />
        {#if fieldErrors.key}<span class="field-error">{fieldErrors.key}</span>{/if}
      </label>
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Connecting…' : 'Connect'}</button>
        {#if editing}<button type="button" class="link" onclick={() => (editing = false)}>Cancel</button>{/if}
      </div>
    </form>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .address {
    display: block;
    overflow-wrap: anywhere;
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
