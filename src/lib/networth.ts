import { extent } from "d3-array";
import { axisBottom, axisLeft, axisRight } from "d3-axis";
import { scaleLinear, scaleOrdinal, scaleTime } from "d3-scale";
import { select } from "d3-selection";
// area aliased: local `const area` below self-references the d3-shape
// area generator inside its own initializer.
import { area as d3Area, curveMonotoneX, line } from "d3-shape";
import { Delaunay } from "d3";
import _ from "lodash";
import tippy from "tippy.js";
import COLORS from "./colors";
import {
  formatCurrency,
  formatCurrencyCrude,
  isMobile,
  now,
  svgUrl,
  tooltip,
  type Legend,
  type Networth
} from "./utils";

function networth(d: Networth) {
  return d.investmentAmount + d.gainAmount - d.withdrawalAmount;
}

function investment(d: Networth) {
  return d.investmentAmount - d.withdrawalAmount;
}

export function renderNetworth(
  points: Networth[],
  element: Element
): { destroy: () => void; legends: Legend[] } {
  const start = _.min(_.map(points, (p) => p.date)),
    end = now();

  const svg = select(element);

  svg.selectAll("*").remove();

  const right = isMobile() ? 10 : 80,
    margin = { top: 15, right: right, bottom: 20, left: 40 },
    width = Math.max(element.parentElement.clientWidth, 800) - margin.left - margin.right,
    height = +svg.attr("height") - margin.top - margin.bottom,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  svg.attr("width", width + margin.left + margin.right);

  const areaKeys = ["gain", "loss"];
  const colors = [COLORS.gain, COLORS.loss];
  const areaScale = scaleOrdinal<string>().domain(areaKeys).range(colors);

  const lineKeys = ["networth", "investment"];
  const lineScale = scaleOrdinal<string>()
    .domain(lineKeys)
    .range([COLORS.primary, COLORS.secondary]);

  const positions = _.flatMap(points, (p) => [
    p.gainAmount + p.investmentAmount - p.withdrawalAmount,
    p.investmentAmount - p.withdrawalAmount
  ]);
  positions.push(0);

  const x = scaleTime().range([0, width]).domain([start, end]),
    y = scaleLinear().range([height, 0]).domain(extent(positions)),
    z = scaleOrdinal<string>(colors).domain(areaKeys);

  const area = (y0: number, y1: (d: Networth) => number) =>
    d3Area<Networth>()
      .curve(curveMonotoneX)
      .x((d) => x(d.date))
      .y0(y0)
      .y1(y1);

  g.append("g")
    .attr("class", "axis x")
    .attr("transform", "translate(0," + height + ")")
    .call(axisBottom(x));

  if (!isMobile()) {
    g.append("g")
      .attr("class", "axis y")
      .attr("transform", `translate(${width},0)`)
      .call(axisRight(y).tickPadding(5).tickFormat(formatCurrencyCrude));
  }

  g.append("g")
    .attr("class", "axis y")
    .call(axisLeft(y).tickSize(-width).tickFormat(formatCurrencyCrude));

  const layer = g.selectAll(".layer").data([points]).enter().append("g").attr("class", "layer");

  const clipAboveID = _.uniqueId("clip-above");
  layer
    .append("clipPath")
    .attr("id", clipAboveID)
    .append("path")
    .attr(
      "d",
      area(height, (d) => {
        return y(d.gainAmount + d.investmentAmount - d.withdrawalAmount);
      })
    );

  const clipBelowID = _.uniqueId("clip-below");
  layer
    .append("clipPath")
    .attr("id", clipBelowID)
    .append("path")
    .attr(
      "d",
      area(0, (d) => {
        return y(d.gainAmount + d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .style("fill", z("gain"))
    .style("opacity", "0.2")
    .attr("clip-path", svgUrl(clipAboveID))
    .attr(
      "d",
      area(0, (d) => {
        return y(d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .attr("clip-path", svgUrl(clipBelowID))
    .style("fill", z("loss"))
    .style("opacity", "0.2")
    .attr(
      "d",
      area(height, (d) => {
        return y(d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .style("stroke", lineScale("investment"))
    .style("stroke-width", "1.5")
    .style("fill", "none")
    .attr(
      "d",
      line<Networth>()
        .curve(curveMonotoneX)
        .x((d) => x(d.date))
        .y((d) => y(investment(d)))
    );

  layer
    .append("path")
    .style("stroke", lineScale("networth"))
    .style("stroke-width", "1.5")
    .style("fill", "none")
    .attr(
      "d",
      line<Networth>()
        .curve(curveMonotoneX)
        .x((d) => x(d.date))
        .y((d) => y(networth(d)))
    );

  const hoverCircle = layer.append("circle").attr("r", "3").attr("fill", "none");
  const t = tippy(hoverCircle.node(), { theme: "light", delay: 0, allowHTML: true });

  const networthVoronoiPoints: Delaunay.Point[] = _.map(points, (d) => [x(d.date), y(networth(d))]);
  const investmentVoronoiPoints: Delaunay.Point[] = _.map(points, (d) => [
    x(d.date),
    y(investment(d))
  ]);
  const voronoi = Delaunay.from(networthVoronoiPoints.concat(investmentVoronoiPoints)).voronoi([
    0,
    0,
    width,
    height
  ]);

  layer
    .append("g")
    .selectAll("path")
    .data(
      points.map((p) => ["networth", p]).concat(points.map((p) => ["investment", p])) as [
        string,
        Networth
      ][]
    )
    .enter()
    .append("path")
    .style("pointer-events", "all")
    .style("fill", "none")
    .attr("d", (_, i) => {
      return voronoi.renderCell(i);
    })
    .on("mouseover", (_, [pointType, d]) => {
      hoverCircle
        .attr("cx", x(d.date))
        .attr("cy", y(pointType == "networth" ? networth(d) : investment(d)))
        .attr("fill", lineScale(pointType));

      t.setProps({
        placement: pointType == "networth" ? "top" : "bottom",
        content: tooltip([
          ["Date", d.date.format("DD MMM YYYY")],
          ["Net Worth", [formatCurrency(networth(d)), "has-text-weight-bold has-text-right"]],
          [
            "Net Investment",
            [formatCurrency(investment(d)), "has-text-weight-bold has-text-right"]
          ],
          ["Gain / Loss", [formatCurrency(d.gainAmount), "has-text-weight-bold has-text-right"]]
        ])
      });
      t.show();
    })
    .on("mouseout", () => {
      t.hide();
      hoverCircle.attr("fill", "none");
    });

  const legends: Legend[] = [
    {
      label: "Net Worth",
      color: lineScale("networth"),
      shape: "line"
    },
    {
      label: "Net Investment",
      color: lineScale("investment"),
      shape: "line"
    },
    {
      label: "Gain",
      color: areaScale("gain"),
      shape: "square"
    },
    {
      label: "Loss",
      color: areaScale("loss"),
      shape: "square"
    }
  ];

  const destroy = () => {
    t.destroy();
  };

  return { destroy, legends };
}
