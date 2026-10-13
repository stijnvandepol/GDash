<script lang="ts">
  import { toast } from './toast.svelte';
  import { api, ApiError, type CustomSource } from './api';
  import { AUTH_KINDS, INTERVALS } from './customSources';
  import { previewOf } from './customWidget';
  import { placeholderProblem, presetInput, type Preset } from './presets';

  // A new custom API from a preset: the household fills in the address's
  // placeholders and the key, GDash makes the API with its named values and
  // asks it once, so a wrong address shows here and not on a TV.
  let {
    preset,
    made,
    onchanged,
    onopen,
    onerror,
  }: {
    preset: Preset;
    /** Custom APIs already made from this preset. */
    made: CustomSource[];
    onchanged: () => Promise<void>;
    /** Opens another integration, here Custom APIs. */
    onopen: (id: string) => void;
    onerror: (err: unknown) => string;
  } = $props();

  // svelte-ignore state_referenced_locally
  let name = $state(preset.source.name);
  let answers = $state<Record<string, string>>({});
  let secret = $state('');
  let user = $state('');
  // svelte-ignore state_referenced_locally
  let intervalSeconds = $state(preset.source.intervalSeconds);
  let problems = $state<Record<string, string>>({});
  let error = $state('');
  let busy = $state(false);
  let saved = $state<CustomSource | null>(null);
  // The answer of the first request: the data, or why it failed.
  let answer = $state<{ data: unknown } | { problem: string } | null>(null);

  const auth = $derived(AUTH_KINDS.find((a) => a.kind === preset.source.auth.kind));
  const ids = $derived(`preset-${preset.id}`);

  function check(): boolean {
    const found: Record<string, string> = {};
    for (const p of preset.placeholders) {
      const problem = placeholderProblem(p.kind, answers[p.key] ?? '');
      if (problem) found[p.key] = problem;
    }
    if (name.trim() === '') found.name = 'Fill this in.';
    if (auth?.secretLabel && secret.trim() === '') found.secret = 'Fill this in.';
    if (preset.source.auth.kind === 'basic' && user.trim() === '') found.user = 'Fill this in.';
    problems = found;
    return Object.keys(found).length === 0;
  }

  async function add(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!check()) return;
    busy = true;
    try {
      const input = { ...presetInput(preset, { name, answers, secret, user }), intervalSeconds };
      const source = await api.addCustomSource(input);
      saved = await api.saveCustomSourceValues(source.id, { values: preset.values, lists: preset.lists });
      toast.success(`${saved.name} is added.`);
      secret = '';
      await onchanged();
      await ask(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length > 0) {
        const fields = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
        problems = { name: fields.name ?? '', secret: fields['auth.secret'] ?? '', user: fields['auth.name'] ?? '' };
        const rest = err.fields.filter((f) => !['name', 'auth.secret', 'auth.name'].includes(f.field));
        error = rest.map((f) => `${f.field}: ${f.message}`).join('; ');
      } else {
        error = onerror(err);
      }
    } finally {
      busy = false;
    }
  }

  async function ask(source: CustomSource) {
    answer = null;
    try {
      answer = { data: (await api.testCustomSource(source.id)).data };
    } catch (err) {
      answer = { problem: err instanceof ApiError && err.status !== 401 ? err.message : onerror(err) };
    }
  }

  function again() {
    saved = null;
    answer = null;
    answers = {};
    name = preset.source.name;
  }
</script>

<section class="card preset" aria-labelledby="{ids}-title">
  <h2 id="{ids}-title">{preset.name}</h2>
  {#if saved}
    <p>
      <strong>{saved.name}</strong> is added as a custom API with
      {saved.values.length === 1 ? '1 value' : `${saved.values.length} values`}{saved.lists.length > 0 ? ` and ${saved.lists.length === 1 ? '1 list' : `${saved.lists.length} lists`}` : ''}.
      Add a Custom API widget to a dashboard and choose them by name.
    </p>
    {#if answer === null}
      <p class="small muted" role="status">Asking {preset.name} for the first time…</p>
    {:else if 'problem' in answer}
      <p class="small warn" role="status">GDash could not read it yet: {answer.problem} Check the address under Custom APIs.</p>
    {:else}
      <p class="small" role="status">{preset.name} answered. What the values show now:</p>
    {/if}
    <ul class="list values">
      {#each saved.values as value (value.name)}
        <li>
          <span class="grow"><strong>{value.name}</strong></span>
          {#if answer && 'data' in answer}<span class="now">{previewOf(answer.data, value.path)}{value.unit ? ` ${value.unit}` : ''}</span>{/if}
        </li>
      {/each}
      {#each saved.lists as list (list.name)}
        <li>
          <span class="grow"><strong>{list.name}</strong> <span class="muted small">list of {list.columns.map((c) => c.name).join(', ')}</span></span>
        </li>
      {/each}
    </ul>
    {#if preset.rule}
      <p class="small muted">
        A rule to try on a widget that shows {preset.rule.for}: {preset.rule.summary}, with the condition
        <code>{preset.rule.when}</code> under Advanced.
      </p>
    {/if}
    <div class="inline">
      <button type="button" class="secondary" onclick={() => onopen('custom')}>Open Custom APIs</button>
      <button type="button" class="link" onclick={again}>Add another</button>
    </div>
  {:else}
    <p class="muted">{preset.description}</p>
    <p class="small muted">
      Makes a custom API that you can change later under Custom APIs.
      <a href={preset.docs} rel="noopener" target="_blank">The API’s documentation</a>.
      {#if made.length > 0}Already added: {made.map((s) => s.name).join(', ')}.{/if}
    </p>
    <form onsubmit={add} novalidate>
      <label>
        Name
        <input bind:value={name} maxlength="60" autocomplete="off" aria-invalid={problems.name ? 'true' : undefined} aria-describedby={problems.name ? `${ids}-name-problem` : undefined} />
        {#if problems.name}<span class="field-error" id="{ids}-name-problem">{problems.name}</span>{/if}
      </label>
      {#each preset.placeholders as p (p.key)}
        <label>
          {p.label}
          <input
            value={answers[p.key] ?? ''}
            oninput={(e) => (answers[p.key] = (e.target as HTMLInputElement).value)}
            placeholder={p.example}
            autocomplete="off"
            spellcheck="false"
            inputmode={p.kind === 'origin' ? 'url' : undefined}
            aria-invalid={problems[p.key] ? 'true' : undefined}
            aria-describedby={problems[p.key] ? `${ids}-${p.key}-problem` : undefined}
          />
          {#if p.hint}<span class="small muted">{p.hint}</span>{/if}
          {#if problems[p.key]}<span class="field-error" id="{ids}-{p.key}-problem">{problems[p.key]}</span>{/if}
        </label>
      {/each}
      {#if preset.source.auth.kind === 'basic'}
        <label>
          {auth?.nameLabel}
          <input bind:value={user} autocomplete="off" spellcheck="false" aria-invalid={problems.user ? 'true' : undefined} aria-describedby={problems.user ? `${ids}-user-problem` : undefined} />
          {#if problems.user}<span class="field-error" id="{ids}-user-problem">{problems.user}</span>{/if}
        </label>
      {/if}
      {#if auth?.secretLabel}
        <label>
          {auth.secretLabel}
          <!-- Not type="password", so browsers do not offer to save it. -->
          <input bind:value={secret} class="masked" autocomplete="off" spellcheck="false" aria-invalid={problems.secret ? 'true' : undefined} aria-describedby={problems.secret ? `${ids}-secret-problem` : undefined} />
          <span class="small muted">{preset.source.secretHint ? `${preset.source.secretHint} ` : ''}Stored encrypted and never shown again.</span>
          {#if problems.secret}<span class="field-error" id="{ids}-secret-problem">{problems.secret}</span>{/if}
        </label>
      {/if}
      <label>
        Refresh
        <select bind:value={intervalSeconds}>
          {#each INTERVALS as i (i.seconds)}<option value={i.seconds}>{i.label}</option>{/each}
        </select>
      </label>
      <div class="inline">
        <button type="submit" disabled={busy}>{busy ? 'Adding…' : `Add ${preset.name}`}</button>
      </div>
    </form>
  {/if}
  {#if error}<p class="error">{error}</p>{/if}
</section>

<style>
  .values {
    max-width: 40rem;
  }
  .now {
    font-family: var(--mono);
    font-size: var(--fs-sm);
    overflow-wrap: anywhere;
  }
  .warn {
    color: var(--warning);
  }
  .masked {
    -webkit-text-security: disc;
  }
</style>
