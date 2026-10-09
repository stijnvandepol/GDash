<script lang="ts">
  import FileButton from './FileButton.svelte';
  import { onMount } from 'svelte';
  import { api, ApiError, type Photo } from './api';

  let { onerror }: { onerror: (err: unknown) => string } = $props();

  // The server keeps at most this many photos (internal/photos.MaxPhotos).
  const MAX_PHOTOS = 50;

  let photos = $state<Photo[]>([]);
  let busy = $state(false);
  let error = $state('');

  onMount(() => void load());

  async function load() {
    try {
      photos = await api.photos();
    } catch (err) {
      error = onerror(err);
    }
  }

  // Uploads one file after the other, so a problem with one photo names it
  // and the rest still arrive.
  async function upload(event: Event) {
    const input = event.target as HTMLInputElement;
    const files = [...(input.files ?? [])];
    input.value = '';
    busy = true;
    error = '';
    const failed: string[] = [];
    for (const file of files) {
      try {
        await api.uploadPhoto(file);
      } catch (err) {
        if (!(err instanceof ApiError) || err.status === 401) {
          error = onerror(err);
          break;
        }
        failed.push(`${file.name}: ${err.fieldMessage('photo') ?? err.message}`);
      }
    }
    if (failed.length > 0) error = failed.join(' · ');
    busy = false;
    await load();
  }

  async function remove(p: Photo) {
    if (!confirm('Remove this photo from GDash?')) return;
    try {
      await api.deletePhoto(p.name);
      await load();
    } catch (err) {
      error = onerror(err);
    }
  }
</script>

<section class="card">
  <h2>Photos</h2>
  <p class="muted">
    For the Photos widget. Photos stay on this server and only go to paired screens. JPEG, PNG or WebP, up to 8 MB
    each, at most {MAX_PHOTOS}.
  </p>
  {#if photos.length > 0}
    <ul class="photos">
      {#each photos as p (p.name)}
        <li>
          <img src="/api/photos/{p.name}" alt="" loading="lazy" />
          <button class="link danger" onclick={() => remove(p)}>Remove</button>
        </li>
      {/each}
    </ul>
  {/if}
  {#if photos.length < MAX_PHOTOS}
    <FileButton label="Add photos" accept="image/jpeg,image/png,image/webp" multiple disabled={busy} onchange={upload} />
  {/if}
  {#if busy}<p class="muted">Uploading…</p>{/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .photos {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
    gap: 10px;
  }
  .photos li {
    display: grid;
    gap: 4px;
    justify-items: start;
  }
  .photos img {
    width: 100%;
    aspect-ratio: 16 / 9;
    object-fit: cover;
    border-radius: 6px;
  }
</style>
