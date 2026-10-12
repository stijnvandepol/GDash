<script lang="ts">
  // What Displays shows before there is one: a TV that is still off, and
  // the two ways a screen gets a dashboard.
</script>

<div class="empty">
  <div class="set" aria-hidden="true">
    <div class="bezel"><div class="blank"></div><span class="led"></span></div>
    <span class="stand"></span>
  </div>
  <div class="text">
    <h2>No displays yet</h2>
    <p class="muted">Add one below. There are two ways to get a dashboard onto a screen:</p>
    <ul>
      <li><strong>A TV with Chromecast or Google TV</strong> on the same network is found on its own. GDash puts the dashboard on it and keeps it there.</li>
      <li><strong>Any other screen</strong>, such as a tablet, a Fire TV or a computer, opens the display's address in its browser. It then shows a code, which you enter here to connect it.</li>
    </ul>
  </div>
</div>

<style>
  .empty {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--s-4) var(--s-6);
    align-items: center;
    padding: var(--s-3) 0;
  }
  @container page (min-width: 700px) {
    .empty {
      grid-template-columns: 240px minmax(0, 1fr);
    }
  }
  .set {
    display: grid;
    justify-items: center;
    max-width: 240px;
  }
  .bezel {
    position: relative;
    width: 100%;
    padding: 6px 6px 14px;
    border-radius: var(--r-lg);
    background: var(--device);
    box-shadow: inset 0 0 0 1px var(--device-edge);
  }
  .blank {
    aspect-ratio: 16 / 9;
    border-radius: 3px;
    background: var(--device-screen);
  }
  .led {
    position: absolute;
    right: 12px;
    bottom: 5px;
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--device-led-off);
  }
  .stand {
    width: 28%;
    height: 6px;
    margin-top: -1px;
    border-radius: 0 0 3px 3px;
    background: var(--device-stand);
  }
  .text {
    display: grid;
    gap: var(--s-2);
    max-width: 62ch;
  }
  ul {
    margin: var(--s-1) 0 0;
    padding-left: 1.1em;
    display: grid;
    gap: var(--s-2);
  }
</style>
