<script lang="ts">
  import { MAX_RULES, TONES, type Rule } from './customWidget';

  let {
    rules,
    mode,
    onchange,
    onfocus,
    oncheck,
  }: {
    rules: Rule[];
    mode: string;
    onchange: (rules: Rule[]) => void;
    /** A rule's condition has focus: a clicked value's path goes there. */
    onfocus: (index: number) => void;
    /** Evaluates a condition against the data shown, or undefined without data. */
    oncheck?: (when: string) => Promise<string>;
  } = $props();

  let checks = $state<Record<number, string>>({});

  function update(index: number, change: Partial<Rule>) {
    const next = rules.map((r, i) => (i === index ? { ...r, ...change } : r));
    // Unset parts are left out, as the server stores them.
    const rule = next[index]!;
    if (!rule.tone) delete rule.tone;
    if (!rule.message) delete rule.message;
    if (!rule.hide) delete rule.hide;
    if (!rule.announce) delete rule.announce;
    onchange(next);
  }

  async function check(index: number) {
    if (!oncheck) return;
    checks = { ...checks, [index]: 'Checking…' };
    checks = { ...checks, [index]: await oncheck(rules[index]!.when) };
  }
</script>

<fieldset class="rules">
  <legend>Rules <span class="muted">(optional)</span></legend>
  <p class="small muted">
    {#if mode === 'list'}
      A rule that reads <code>row</code>, such as <code>row.price &gt; 2</code>, is checked for every item.
    {:else if mode === 'value'}
      <code>value</code> is the value shown, as in <code>value &gt; 25 and value &lt; 30</code>.
    {:else}
      Paths start at the top of the answer, as in <code>data.temp &gt; 25</code>.
    {/if}
    The most serious colour of the matching rules wins; the first matching message is shown.
  </p>
  {#each rules as rule, i (i)}
    <div class="rule">
      <label>
        When
        <input
          value={rule.when}
          maxlength="500"
          spellcheck="false"
          autocomplete="off"
          placeholder={mode === 'list' ? 'row.price > 2' : 'value > 25'}
          onfocus={() => onfocus(i)}
          oninput={(e) => update(i, { when: (e.target as HTMLInputElement).value })}
        />
      </label>
      <div class="actions">
        <label>
          Colour
          <select value={rule.tone ?? ''} onchange={(e) => update(i, { tone: (e.target as HTMLSelectElement).value })}>
            {#each TONES as t (t.tone)}<option value={t.tone}>{t.label}</option>{/each}
          </select>
        </label>
        <label class="grow-field">
          Message
          <input
            value={rule.message ?? ''}
            maxlength="120"
            oninput={(e) => update(i, { message: (e.target as HTMLInputElement).value })}
          />
        </label>
      </div>
      <div class="actions">
        <label class="check">
          <input type="checkbox" checked={rule.hide ?? false} onchange={(e) => update(i, { hide: (e.target as HTMLInputElement).checked })} />
          {mode === 'list' ? 'Hide' : 'Hide the data'}
        </label>
        <label class="check">
          <input
            type="checkbox"
            checked={rule.announce ?? false}
            onchange={(e) => update(i, { announce: (e.target as HTMLInputElement).checked })}
          />
          Announce the message on every display when this starts
        </label>
        {#if oncheck}<button type="button" class="link" onclick={() => check(i)}>Check</button>{/if}
        <button type="button" class="link danger" onclick={() => onchange(rules.filter((_, j) => j !== i))}>Remove</button>
      </div>
      {#if checks[i]}<p class="small mono">{checks[i]}</p>{/if}
    </div>
  {/each}
  {#if rules.length < MAX_RULES}
    <button type="button" class="link" onclick={() => onchange([...rules, { when: '', tone: 'warn' }])}>Add a rule</button>
  {/if}
</fieldset>

<style>
  .rules {
    display: grid;
    gap: var(--s-3);
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    font-weight: 550;
  }
  .rule {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: var(--s-2) var(--s-3);
  }
  .grow-field {
    flex: 1 1 12rem;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 400;
  }
  code {
    font-family: var(--mono);
  }
</style>
