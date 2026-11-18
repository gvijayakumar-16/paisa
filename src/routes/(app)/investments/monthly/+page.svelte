<script lang="ts">
  import { onMount } from "svelte";
  import _ from "lodash";
  import dayjs from "dayjs";
  import { ajax, formatCurrency, type Price, type Posting } from "$lib/utils";
  import {
    investmentHoldingsAt,
    type InvestmentHolding,
    loadInvestmentPrices,
    renderInvestmentHoldingsBreakdown,
    isStockPosting,
    stockName
  } from "$lib/investment_breakdown";
  import { renderCurrentExpensesBreakdown, colorScale } from "$lib/expense/monthly";
  import { month, setAllowedDateRange } from "../../../../store";
  import LevelItem from "$lib/components/LevelItem.svelte";
  import COLORS, { generateColorScheme } from "$lib/colors";
  import ZeroState from "$lib/components/ZeroState.svelte";
  import BoxLabel from "$lib/components/BoxLabel.svelte";

  let renderer: (ps: Posting[]) => void,
    valueRenderer: (hs: InvestmentHolding[]) => void,
    stockRenderer: (hs: InvestmentHolding[]) => void,
    prices: Record<string, Price[]>,
    grouped: Record<string, Posting[]>,
    total = "";

  $: if (grouped) {
    const investments = grouped[$month] || [];
    total = formatCurrency(_.sumBy(investments, (p) => p.amount));
    renderer(investments);
  }

  $: if (stockRenderer && grouped) {
    stockRenderer(
      investmentHoldingsAt(
        (grouped[$month] || []).filter(isStockPosting),
        prices || {},
        dayjs($month, "YYYY-MM").endOf("month"),
        stockName
      )
    );
  }

  $: if (grouped && prices && valueRenderer) {
    valueRenderer(
      investmentHoldingsAt(grouped[$month] || [], prices, dayjs($month, "YYYY-MM").endOf("month"))
    );
  }

  onMount(async () => {
    ({
      month_wise: { investments: grouped }
    } = await ajax("/api/expense"));

    const all = _.flatten(_.values(grouped));
    setAllowedDateRange(_.map(all, (e) => e.date));
    renderer = renderCurrentExpensesBreakdown(
      colorScale(all),
      "#d3-current-month-investment-breakdown",
      "Assets"
    );
    const z = colorScale(all);
    stockRenderer = renderInvestmentHoldingsBreakdown(
      "#d3-current-month-stock-cumulative",
      generateColorScheme(_.uniq(all.filter(isStockPosting).map(stockName)).sort()),
      "invested"
    );
    valueRenderer = renderInvestmentHoldingsBreakdown("#d3-current-month-investment-value", z);
    prices = await loadInvestmentPrices();
  });
</script>

<section class="section tab-expense">
  <div class="container is-fluid">
    <div class="columns is-flex-wrap-wrap">
      <div class="column is-3">
        <nav class="level">
          <LevelItem title="Net Investment" value={total} color={COLORS.secondary} />
        </nav>
      </div>
      <div class="column is-9">
        <div class="px-3 box" style="height: 100%">
          <ZeroState item={grouped?.[$month]}>
            <strong>Oops!</strong> You have no investments this month.
          </ZeroState>
          <svg id="d3-current-month-investment-breakdown" width="100%" />
        </div>
      </div>
      <div class="column is-12">
        <div class="px-3 box">
          <p class="has-text-grey is-size-7 pt-3">
            Value at month end of the units bought this month
          </p>
          <svg id="d3-current-month-investment-value" width="100%" />
        </div>
      </div>
      <div class="column is-12">
        <div class="px-3 box">
          <p class="has-text-grey is-size-7 pt-3">Stock investments made this month</p>
          <svg id="d3-current-month-stock-cumulative" width="100%" />
        </div>
      </div>
    </div>
    <BoxLabel text="Monthly Investments" />
  </div>
</section>
