<script lang="ts">
  import AssetsBalance from "$lib/components/AssetsBalance.svelte";
  import { ajax, type AssetBreakdown } from "$lib/utils";
  import { downloadAssets } from "$lib/export";
  import _ from "lodash";
  import { onMount } from "svelte";

  let breakdowns: Record<string, AssetBreakdown> = {};

  onMount(async () => {
    ({ asset_breakdowns: breakdowns } = await ajax("/api/assets/balance"));
  });
</script>

<section class="section pb-0">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 pb-0">
        <div class="is-flex is-justify-content-flex-end mb-2">
          <a on:click={() => downloadAssets(breakdowns)}>
            <span class="icon is-small">
              <i class="fa-solid fa-file-arrow-down"></i>
            </span>
            download
          </a>
        </div>
        <AssetsBalance {breakdowns} />
      </div>
    </div>
  </div>
</section>
