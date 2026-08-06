<script lang="ts">
  import { sync } from "$lib/sync";
  import { isLoggedIn, isMobile, logout } from "$lib/utils";
  import { refresh } from "../../store";
  import { obscure } from "../../persisted_store";
  import { goto } from "$app/navigation";

  async function syncWithLoader(request: Record<string, any>) {
    try {
      await sync(request);
    } finally {
      refresh();
    }
  }

  const obscureId = "obscure";
  let last = $obscure;
  obscure.subscribe(() => {
    if ($obscure === last) return;

    refresh();
  });

  function doLogout() {
    logout();
    goto("/login");
  }

  let showLogout = isLoggedIn();
</script>

<!-- ponytail: DaisyUI pilot migration (paisa/pilot-css-migration). Bulma's
     is-hoverable had no DaisyUI equivalent, so this opens on click/focus
     instead of hover - a deliberate behavior change, not an oversight. -->
<div class="du-dropdown ml-2 {isMobile() ? '' : 'du-dropdown-end'}">
  <div
    tabindex="0"
    role="button"
    class="du-btn du-btn-ghost du-btn-circle du-btn-sm"
    aria-haspopup="true"
  >
    <span class="icon">
      <i class="fas fa-ellipsis-vertical" />
    </span>
  </div>
  <ul
    tabindex="0"
    class="du-dropdown-content du-menu bg-base-100 du-rounded-box z-10 w-64 p-2 shadow"
    id="dropdown-menu4"
    role="menu"
  >
    <li>
      <a on:click={(_e) => syncWithLoader({ journal: true })} class="flex items-center gap-2">
        <span class="icon is-small">
          <i class="fa-regular fa-file-lines" />
        </span>
        <span>Sync Journal</span>
      </a>
    </li>
    <li>
      <a on:click={(_e) => syncWithLoader({ prices: true })} class="flex items-center gap-2">
        <span class="icon is-small">
          <i class="fas fa-dollar-sign" />
        </span>
        <span>Update Prices</span>
      </a>
    </li>
    <li>
      <a on:click={(_e) => syncWithLoader({ portfolios: true })} class="flex items-center gap-2">
        <span class="icon is-small">
          <i class="fas fa-layer-group" />
        </span>
        <span>Update Mutual Fund Portfolios</span>
      </a>
    </li>
    <div class="du-divider my-1" />
    <li>
      <a class="flex items-center gap-2">
        <label for={obscureId} class="cursor-pointer w-full inline-flex items-center gap-2">
          <input bind:checked={$obscure} id={obscureId} type="checkbox" class="is-hidden" />
          <span class="icon is-small">
            <i class="fas {$obscure ? 'fa-eye-slash' : 'fa-eye'}" />
          </span>
          <span>{$obscure ? "Show" : "Hide"} numbers</span>
        </label>
      </a>
    </li>
    {#if showLogout}
      <div class="du-divider my-1" />
      <li>
        <a on:click={(_e) => doLogout()} class="flex items-center gap-2">
          <span class="icon is-small">
            <i class="fas fa-arrow-right-from-bracket" />
          </span>
          <span>Logout</span>
        </a>
      </li>
    {/if}
  </ul>
</div>
