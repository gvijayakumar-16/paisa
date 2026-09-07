import type { AssetBreakdown, BalancedPosting } from "./utils";
import Papa from "papaparse";

export function downloadAssets(breakdowns: Record<string, AssetBreakdown>) {
  const ALLOWED_PREFIXES = ["Assets:Debt", "Assets:MutualFund", "Assets:Stocks"];
  const rows = Object.values(breakdowns)
    .filter(
      (b) =>
        ALLOWED_PREFIXES.some((p) => b.group.startsWith(p)) && b.balanceUnits >= 1
    )
    .map((b) => ({
      Account: b.group,
      "Investment Amount": b.investmentAmount,
      "Withdrawal Amount": b.withdrawalAmount,
      "Balance Units": b.balanceUnits,
      "Avg Buy Price": b.averageBuyPrice,
      "Market Value": b.marketAmount,
      Change: b.gainAmount,
      XIRR: b.xirr,
      "Absolute Return": b.absoluteReturn
    }));

  const csv = Papa.unparse(rows);
  const link = document.createElement("a");
  link.href = window.URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8;" }));
  link.download = "paisa-assets-balance.csv";
  link.click();
}

export function download(balancedPostings: BalancedPosting[]) {
  const rows = balancedPostings.map((balancedPosting) => {
    return {
      TransactionID: balancedPosting.from.transaction_id,
      Date: balancedPosting.from.date.toISOString(),
      Payee: balancedPosting.from.payee,
      FromAccount: balancedPosting.from.account,
      FromQuantity: balancedPosting.from.quantity,
      FromAmount: balancedPosting.from.amount,
      FromCommodity: balancedPosting.from.commodity,
      ToAccount: balancedPosting.to.account,
      ToQuantity: balancedPosting.to.quantity,
      ToAmount: balancedPosting.to.amount,
      ToCommodity: balancedPosting.to.commodity
    };
  });

  const csv = Papa.unparse(rows);
  const downloadLink = document.createElement("a");
  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  downloadLink.href = window.URL.createObjectURL(blob);
  downloadLink.download = "paisa-transactions.csv";
  downloadLink.click();
}
