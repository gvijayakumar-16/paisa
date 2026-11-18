<script lang="ts">
  import { extent } from "d3-array";
  import { onMount } from "svelte";
  import _ from "lodash";
  import {
    ajax,
    formatCurrency,
    beginningOfFinancialYear,
    now,
    type Price,
    type Posting
  } from "$lib/utils";
  import {
    investmentHoldingsAt,
    type InvestmentHolding,
    loadInvestmentPrices,
    renderInvestmentHoldingsBreakdown,
    isStockPosting,
    stockName
  } from "$lib/investment_breakdown";
  import { renderCurrentExpensesBreakdown } from "$lib/expense/yearly";
  import { colorScale } from "$lib/expense/monthly";
  import { dateMin, dateMax, year } from "../../../../store";
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
    const investments = grouped[$year] || [];
    total = formatCurrency(_.sumBy(investments, (p) => p.amount));
    renderer(investments);
  }

  $: if (stockRenderer && grouped) {
    stockRenderer(
      investmentHoldingsAt(
        (grouped[$year] || []).filter(isStockPosting),
        prices || {},
        yearEnd(grouped[$year]),
        stockName
      )
    );
  }

  $: if (grouped && prices && valueRenderer) {
    valueRenderer(investmentHoldingsAt(grouped[$year] || [], prices, yearEnd(grouped[$year])));
  }

  function yearEnd(ps: Posting[] = []) {
    return ps.length
      ? beginningOfFinancialYear(ps[0].date).add(1, "year").subtract(1, "day")
      : now();
  }

  onMount(async () => {
    ({
      year_wise: { investments: grouped }
    } = await ajax("/api/expense"));

    const all = _.flatten(_.values(grouped));
    const [start, end] = extent(_.map(all, (e) => e.date));
    if (start) {
      dateMin.set(start);
      dateMax.set(end);
    }
    renderer = renderCurrentExpensesBreakdown(
      colorScale(all),
      "#d3-current-year-investment-breakdown",
      "Assets"
    );
    const z = colorScale(all);
    stockRenderer = renderInvestmentHoldingsBreakdown(
      "#d3-current-year-stock-cumulative",
      generateColorScheme(_.uniq(all.filter(isStockPosting).map(stockName)).sort()),
      "invested"
    );
    valueRenderer = renderInvestmentHoldingsBreakdown("#d3-current-year-investment-value", z);
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
          <ZeroState item={grouped?.[$year]}>
            <strong>Oops!</strong> You have no investments this year.
          </ZeroState>
          <svg id="d3-current-year-investment-breakdown" width="100%" />
        </div>
      </div>
      <div class="column is-12">
        <div class="px-3 box">
          <p class="has-text-grey is-size-7 pt-3">
            Value at year end of the units bought this year
          </p>
          <svg id="d3-current-year-investment-value" width="100%" />
        </div>
      </div>
      <div class="column is-12">
        <div class="px-3 box">
          <p class="has-text-grey is-size-7 pt-3">Stock investments made this year</p>
          <svg id="d3-current-year-stock-cumulative" width="100%" />
        </div>
      </div>
    </div>
    <BoxLabel text="Yearly Investments" />
  </div>
</section>
