import { max } from "d3-array";
import { axisBottom, axisLeft } from "d3-axis";
import { scaleBand, scaleLinear, type ScaleOrdinal } from "d3-scale";
import { select } from "d3-selection";
import chroma from "chroma-js";
import _ from "lodash";
import type { Dayjs } from "dayjs";
import {
  ajax,
  formatCurrency,
  formatCurrencyCrude,
  formatPercentage,
  rem,
  secondName,
  skipTicks,
  tooltip,
  type Posting,
  type Price
} from "$lib/utils";
import { iconify } from "$lib/icon";
import { formatBreakdownLabel } from "$lib/expense";

export interface InvestmentHolding {
  category: string;
  invested: number;
  units: number;
  value: number;
}

export async function loadInvestmentPrices(): Promise<Record<string, Price[]>> {
  const { prices } = await ajax("/api/price");
  return _.mapValues(prices, (ps) => _.sortBy(ps, (p) => p.date.valueOf()));
}

// Value of only these postings' units at `end`, using the latest price on or before it.
// Falls back to cost when a commodity has no price yet, like the backend does.
export function investmentHoldingsAt(
  postings: Posting[],
  prices: Record<string, Price[]>,
  end: Dayjs,
  categoryOf: (p: Posting) => string = (p) => secondName(p.account)
): InvestmentHolding[] {
  return _.map(_.groupBy(postings, categoryOf), (ps, category) => ({
    category,
    invested: _.sumBy(ps, (p) => p.amount),
    units: _.sumBy(ps, (p) => p.quantity),
    value: _.sumBy(ps, (p) => {
      const price = _.findLast(prices[p.commodity], (x) => !x.date.isAfter(end, "day"));
      return price && price.value != 0 ? p.quantity * price.value : p.amount;
    })
  }));
}

// metric "invested" sizes bars by cost, labelled with share of total, for cumulative views.
export function renderInvestmentHoldingsBreakdown(
  id: string,
  z: ScaleOrdinal<string, string, never>,
  metric: "value" | "invested" = "value"
) {
  const BAR_HEIGHT = rem(20);
  const svg = select(id),
    margin = { top: 0, right: rem(25), bottom: rem(20), left: rem(100) };

  return (holdings: InvestmentHolding[]) => {
    svg.selectAll("*").remove();
    const g = svg.append("g").attr("transform", `translate(${margin.left},${margin.top})`);
    const sorted = _.sortBy(holdings, (h) => h[metric]);
    const total = _.sumBy(sorted, (h) => h[metric]);
    const label = (h: InvestmentHolding) => {
      if (metric == "invested") return formatBreakdownLabel(h.invested, total);
      const gain = h.value - h.invested;
      const pct =
        h.invested != 0
          ? ` (${gain >= 0 ? "+" : ""}${formatPercentage(gain / h.invested, 1)})`
          : "";
      return `${formatCurrency(h.value)}${pct}`;
    };
    const textWidth = Math.max(rem(190), (_.max(sorted.map((h) => label(h).length)) || 0) * 8 + 16);
    const width = Math.max(
      0,
      document.getElementById(id.substring(1)).parentElement.clientWidth -
        margin.left -
        margin.right -
        textWidth
    );
    const height = BAR_HEIGHT * sorted.length;
    svg.attr("height", height + margin.top + margin.bottom);

    const x = scaleLinear()
      .range([0, width])
      .domain([0, max(sorted, (h) => h[metric]) || 0]);
    const y = scaleBand()
      .paddingInner(0.1)
      .paddingOuter(0)
      .domain(sorted.map((h) => h.category))
      .range([height, 0]);

    g.append("g")
      .attr("class", "axis y")
      .attr("transform", `translate(0,${height})`)
      .call(
        axisBottom(x)
          .tickSize(-height)
          .tickFormat(skipTicks(60, x, formatCurrencyCrude))
      );
    g.append("g")
      .attr("class", "axis y dark")
      .call(axisLeft(y).tickFormat((c) => iconify(c, { group: "Assets", suffix: true })));

    const tip = (h: InvestmentHolding) =>
      tooltip(
        [
          [
            "Units",
            [h.units.toLocaleString(undefined, { maximumFractionDigits: 4 }), "has-text-right"]
          ],
          ["Invested", [formatCurrency(h.invested), "has-text-weight-bold has-text-right"]],
          ["Value", [formatCurrency(h.value), "has-text-weight-bold has-text-right"]]
        ],
        { total: formatCurrency(h.value - h.invested), header: `${h.category} gain` }
      );

    g.append("g")
      .selectAll("rect")
      .data(sorted)
      .join("rect")
      .attr("fill", (h) => z(h.category))
      .attr("data-tippy-content", tip)
      .attr("x", 0)
      .attr("y", (h) => y(h.category))
      .attr("width", (h) => Math.max(0, x(h[metric])))
      .attr("height", y.bandwidth());

    g.append("g")
      .selectAll("text")
      .data(sorted)
      .join("text")
      .attr("text-anchor", "end")
      .attr("dominant-baseline", "middle")
      .attr("x", width + textWidth)
      .attr("y", (h) => y(h.category) + y.bandwidth() / 2)
      .style("white-space", "pre")
      .style("font-size", "0.928rem")
      .style("font-weight", "bold")
      .style("fill", (h) => chroma(z(h.category)).darken(0.8).hex())
      .attr("class", "is-family-monospace")
      .text(label);
  };
}

// Stocks are the Assets:Stocks:* accounts; each stock is the account below that.
export const isStockPosting = (p: Posting) => /^stocks?$/i.test(secondName(p.account));
export const stockName = (p: Posting) => p.account.split(":").slice(2).join(":") || p.account;
