import { map, max } from "d3-array";
import { axisBottom, axisLeft, axisRight } from "d3-axis";
import { partition, stratify as d3Stratify, treemap, type HierarchyNode } from "d3-hierarchy";
import { scaleBand, scaleLinear, scaleOrdinal, scaleThreshold, scaleTime, type ScaleOrdinal } from "d3-scale";
import { select } from "d3-selection";
import { curveLinear, line as d3Line } from "d3-shape";
import type dayjs from "dayjs";
import _ from "lodash";
import {
  type Aggregate,
  type AllocationTarget,
  formatCurrency,
  formatFloat,
  lastName,
  parentName,
  secondName,
  tooltip,
  skipTicks,
  rem,
  now,
  type Legend,
  darkenOrLighten
} from "./utils";
import COLORS, { generateColorScheme } from "./colors";
import chroma from "chroma-js";

export function renderAllocationTarget(
  allocationTargets: AllocationTarget[],
  color: ScaleOrdinal<string, string>
) {
  const id = "#d3-allocation-target";

  if (_.isEmpty(allocationTargets)) {
    return;
  }
  allocationTargets = _.sortBy(allocationTargets, (t) => t.name);
  const BAR_HEIGHT = rem(25);
  const svg = select(id),
    margin = { top: rem(20), right: rem(20), bottom: rem(10), left: rem(150) },
    fullWidth = Math.max(document.getElementById(id.substring(1)).parentElement.clientWidth, 1000),
    width = fullWidth - margin.left - margin.right,
    height = allocationTargets.length * BAR_HEIGHT * 2,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");
  svg.attr("height", height + margin.top + margin.bottom);

  svg.attr("width", fullWidth);

  const keys = ["target", "current"];
  const colorKeys = ["target", "current", "diff"];
  const colors = [COLORS.primary, COLORS.secondary, COLORS.diff];

  const y = scaleBand().range([0, height]).paddingInner(0).paddingOuter(0);
  y.domain(allocationTargets.map((t) => t.name));

  const y1 = scaleBand()
    .range([0, y.bandwidth()])
    .domain(keys)
    .paddingInner(0)
    .paddingOuter(0.1);

  const z = scaleOrdinal<string>(colors).domain(colorKeys);

  const z1 = scaleThreshold<number, string>()
    .domain([5, 10, 15])
    .range([COLORS.gain, COLORS.warn, COLORS.loss, COLORS.loss]);

  const maxX = _.chain(allocationTargets)
    .flatMap((t) => [t.current, t.target])
    .max()
    .value();
  const targetWidth = rem(400);
  const targetMargin = rem(20);
  const textGroupWidth = rem(150);
  const textGroupMargin = rem(20);
  const textGroupZero = targetWidth + targetMargin;

  const x = scaleLinear().range([textGroupZero + textGroupWidth + textGroupMargin, width]);
  x.domain([0, maxX]);
  const x1 = scaleLinear().range([0, targetWidth]).domain([0, maxX]);

  g.append("line")
    .classed("svg-grey-lightest", true)
    .attr("x1", 0)
    .attr("y1", height)
    .attr("x2", width)
    .attr("y2", height);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Target")
    .attr("text-anchor", "end")

    .attr("x", textGroupZero + (textGroupWidth * 1) / 3)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Current")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + (textGroupWidth * 2) / 3)
    .attr("y", -5);

  g.append("text")
    .classed("svg-text-grey", true)
    .text("Diff")
    .attr("text-anchor", "end")
    .attr("x", textGroupZero + textGroupWidth)
    .attr("y", -5);

  g.append("g")
    .attr("class", "axis y")
    .attr("transform", "translate(0," + height + ")")
    .call(
      axisBottom(x1)
        .tickSize(-height)
        .tickFormat(skipTicks(40, x, (n: number) => formatFloat(n, 0)))
    );

  g.append("g").attr("class", "axis y dark").call(axisLeft(y));

  const textGroup = g
    .append("g")
    .selectAll("g")
    .data(allocationTargets)
    .enter()
    .append("g")
    .attr("class", "inline-text");

  textGroup
    .append("line")
    .classed("svg-grey-lightest", true)
    .attr("x1", 0)
    .attr("y1", (t) => y(t.name))
    .attr("x2", width)
    .attr("y2", (t) => y(t.name));

  textGroup
    .append("text")
    .text((t) => formatFloat(t.target))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("target"))
    .attr("x", textGroupZero + (textGroupWidth * 1) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatFloat(t.current))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", z("current"))
    .attr("x", textGroupZero + (textGroupWidth * 2) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  textGroup
    .append("text")
    .text((t) => formatFloat(t.current - t.target))
    .attr("text-anchor", "end")
    .attr("dominant-baseline", "middle")
    .style("fill", (t) =>
      chroma(z1(Math.abs(t.current - t.target)))
        .darken()
        .hex()
    )
    .attr("x", textGroupZero + (textGroupWidth * 3) / 3)
    .attr("y", (t) => y(t.name) + y.bandwidth() / 2);

  const groups = g
    .append("g")
    .selectAll("g.group")
    .data(allocationTargets)
    .enter()
    .append("g")
    .attr("class", "group");

  groups
    .append("rect")
    .attr("fill", (d) => z1(Math.abs(d.target - d.current)))
    .attr("x", x1(0))
    .attr("y", (d) => y(d.name) + y.bandwidth() / 4)
    .attr("height", y.bandwidth() / 2)
    .attr("width", (d) => x1(d.current));

  groups
    .append("line")
    .attr("stroke-width", 3)
    .attr("stroke-linecap", "round")
    .attr("stroke", z("target"))
    .attr("x1", (d) => x1(d.target))
    .attr("x2", (d) => x1(d.target))
    .attr("y1", (d) => y(d.name) + y.bandwidth() / 8)
    .attr("y2", (d) => y(d.name) + (y.bandwidth() / 8) * 7);

  groups
    .append("polygon")
    .attr(
      "transform",
      (d) => "translate(" + x1(d.target) + "," + (y(d.name) + y.bandwidth() / 8) + ")"
    )
    .attr("points", "0 0, 0 15, 20 6")
    .attr("fill", z("target"));

  const paddingTop = (y1.range()[1] - y1.bandwidth() * 2) / 2;
  select("#d3-allocation-target-treemap")
    .append("div")
    .style("height", height + margin.top + margin.bottom + "px")
    .style("position", "absolute")
    .style("width", "100%")
    .selectAll("div")
    .data(allocationTargets)
    .enter()
    .append("div")
    .style("position", "absolute")
    .style("left", margin.left + x(0) + "px")
    .style("top", (t) => margin.top + y(t.name) + paddingTop + "px")
    .style("height", y1.bandwidth() * 2 + "px")
    .style("width", x.range()[1] - x.range()[0] + "px")
    .append("div")
    .style("position", "relative")
    .style("height", y1.bandwidth() * 2 + "px")
    .each(function (t) {
      renderPartition(this, t.aggregates, treemap(), color, {
        margin: { top: 0, right: 0, bottom: 0, left: 0 }
      });
    });
}

export function renderAllocation(
  aggregates: Record<string, Aggregate>,
  color: ScaleOrdinal<string, string>
) {
  renderPartition(
    document.getElementById("d3-allocation-category"),
    aggregates,
    partition(),
    color
  );
  renderPartition(document.getElementById("d3-allocation-value"), aggregates, treemap(), color);
}

function renderPartition(
  element: HTMLElement,
  aggregates: Record<string, Aggregate>,
  hierarchy: any,
  color: ScaleOrdinal<string, string>,
  options = { margin: { top: 0, right: 20, bottom: 0, left: 0 } }
) {
  if (_.isEmpty(aggregates)) {
    return;
  }

  const div = select(element),
    margin = options.margin,
    width = element.parentElement.clientWidth - margin.left - margin.right,
    height = +div.style("height").replace("px", "") - margin.top - margin.bottom;

  const percent = (d: HierarchyNode<Aggregate>) => {
    return formatFloat((d.value / root.value) * 100) + "%";
  };

  const stratify = d3Stratify<Aggregate>()
    .id((d) => d.account)
    .parentId((d) => parentName(d.account));

  const partition = hierarchy.size([width, height]).round(true);

  const root = stratify(_.sortBy(aggregates, (a) => a.account))
    .sum((a) => a.market_amount)
    .sort(function (a, b) {
      return b.height - a.height || b.value - a.value;
    });

  partition(root);

  const cell = div
    .selectAll(".node")
    .data(root.descendants())
    .enter()
    .append("div")
    .attr("class", "node")
    .attr("data-tippy-content", (d) => {
      return tooltip([
        ["Account", [d.id, "has-text-right"]],
        ["Market Value", [formatCurrency(d.value), "has-text-weight-bold has-text-right"]],
        ["Percentage", [percent(d), "has-text-weight-bold has-text-right"]]
      ]);
    })
    .style("top", (d: any) => d.y0 + "px")
    .style("left", (d: any) => d.x0 + "px")
    .style("width", (d: any) => d.x1 - d.x0 + "px")
    .style("height", (d: any) => d.y1 - d.y0 + "px")
    .style("background", (d) => color(d.id))
    .style("color", (d) => darkenOrLighten(color(d.id)));

  cell
    .append("p")
    .attr("class", "heading has-text-weight-bold")
    .text((d) => lastName(d.id));

  cell
    .append("p")
    .attr("class", "heading has-text-weight-bold")
    .style("font-size", ".5 rem")
    .text(percent);
}

export function renderAllocationTimeline(
  aggregatesTimeline: { [key: string]: Aggregate }[]
): Legend[] {
  const timeline = _.map(aggregatesTimeline, (aggregates) => {
    return _.chain(aggregates)
      .values()
      .filter((a) => a.market_amount != 0)
      .groupBy((a) => secondName(a.account))
      .map((aggregates, group) => {
        return {
          date: aggregates[0].date,
          account: group,
          market_amount: _.sum(_.map(aggregates, (a) => a.market_amount)),
          timestamp: aggregates[0].date
        };
      })
      .value();
  });
  const assets = _.chain(timeline)
    .last()
    .map((a) => a.account)
    .sort()
    .value();

  const defaultValues = _.zipObject(
    assets,
    _.map(assets, () => 0)
  );
  const start = timeline[0]?.[0]?.timestamp,
    end = now();

  if (!start) {
    return [];
  }

  interface Point {
    date: dayjs.Dayjs;
    [key: string]: number | dayjs.Dayjs;
  }
  const points: Point[] = [];
  _.each(timeline, (aggregates) => {
    const total = _.sum(_.map(aggregates, (a) => a.market_amount));
    if (total == 0) {
      return;
    }
    const kvs = _.map(aggregates, (a) => [a.account, (a.market_amount / total) * 100]);
    points.push(
      _.merge(
        {
          date: aggregates[0].timestamp
        },
        defaultValues,
        _.fromPairs(kvs)
      )
    );
  });

  const svg = select("#d3-allocation-timeline"),
    margin = { top: 40, right: 60, bottom: 20, left: 35 },
    width =
      document.getElementById("d3-allocation-timeline").parentElement.clientWidth -
      margin.left -
      margin.right,
    height = +svg.attr("height") - margin.top - margin.bottom,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  const x = scaleTime().range([0, width]).domain([start, end]),
    y = scaleLinear()
      .range([height, 0])
      .domain([0, max(map(points, (p) => max(_.values(_.omit(p, "date")))))]),
    z = generateColorScheme(assets);

  const line = (group: string) =>
    d3Line<Point>()
      .curve(curveLinear)
      .defined((p, i) => (p[group] as number) > 0 || (points[i + 1]?.[group] as number) > 0)
      .x((p) => x(p.date))
      .y((p) => y(p[group]));

  g.append("g")
    .attr("class", "axis x")
    .attr("transform", "translate(0," + height + ")")
    .call(axisBottom(x));

  g.append("g")
    .attr("class", "axis y")
    .call(
      axisLeft(y)
        .tickSize(-width)
        .tickFormat((y) => `${y}%`)
    );
  g.append("g")
    .attr("class", "axis y")
    .attr("transform", `translate(${width},0)`)
    .call(axisRight(y).tickFormat((y) => `${y}%`));

  const layer = g.selectAll(".layer").data(assets).enter().append("g").attr("class", "layer");

  layer
    .append("path")
    .attr("fill", "none")
    .attr("stroke", (group) => z(group))
    .attr("stroke-width", "2")
    .attr("d", (group) => line(group)(points));

  return assets.map((a) => {
    return {
      label: a,
      color: z(a),
      shape: "square"
    };
  });
}
