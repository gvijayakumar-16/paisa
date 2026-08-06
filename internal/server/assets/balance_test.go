package assets

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB opens a fresh in-memory sqlite DB and clears every
// sync.Once-guarded package-level cache that ComputeBreakdowns transitively
// reads through (service.IsInterest, transaction.GetById, ...) - those
// caches are process-global, so a stale one from an earlier test would
// leak into this one otherwise.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	assert.NoError(t, err)
	model.AutoMigrate(db)

	service.ClearInterestCache()
	service.ClearPriceCache()
	transaction.ClearCache()

	return db
}

func loadTestConfig(t *testing.T) {
	err := config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: main.db\ndefault_currency: INR\n"), "")
	assert.NoError(t, err)
}

func mustDate(t *testing.T, s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	assert.NoError(t, err)
	return d
}

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	assert.NoError(t, err)
	return d
}

// createPostings inserts the postings and returns them (Create populates ID).
func createPostings(t *testing.T, db *gorm.DB, ps []posting.Posting) []posting.Posting {
	for i := range ps {
		assert.NoError(t, db.Create(&ps[i]).Error)
	}
	return ps
}

func TestComputeBreakdown_ExcludesCheckingAndNegativeFromInvestment(t *testing.T) {
	loadTestConfig(t)
	db := setupTestDB(t)

	ps := createPostings(t, db, []posting.Posting{
		{TransactionID: "t1", Account: "Assets:Checking", Commodity: "INR", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "-1000"), Quantity: mustDecimal(t, "-1000"), MarketAmount: mustDecimal(t, "-1000")},
		{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "1000"), Quantity: mustDecimal(t, "10"), MarketAmount: mustDecimal(t, "1200")},
	})

	bd := ComputeBreakdown(db, ps, true, "Assets:Equity:AAPL")
	// only the AAPL leg should count; the checking leg is excluded by
	// utils.IsCheckingAccount, and ComputeBreakdown is called per-group
	// with already-filtered postings, so pass only the AAPL posting here.
	bdAAPL := ComputeBreakdown(db, []posting.Posting{ps[1]}, true, "Assets:Equity:AAPL")
	assert.True(t, bdAAPL.InvestmentAmount.Equal(mustDecimal(t, "1000")), "investment amount: %s", bdAAPL.InvestmentAmount)
	assert.True(t, bdAAPL.MarketAmount.Equal(mustDecimal(t, "1200")), "market amount: %s", bdAAPL.MarketAmount)
	assert.True(t, bdAAPL.BalanceUnits.Equal(mustDecimal(t, "10")), "balance units: %s", bdAAPL.BalanceUnits)
	assert.True(t, bdAAPL.AverageBuyPrice.Equal(mustDecimal(t, "100")), "average buy price: %s", bdAAPL.AverageBuyPrice)

	// bd (computed with both postings) should still exclude the checking
	// leg from InvestmentAmount even though it wasn't pre-filtered.
	assert.True(t, bd.InvestmentAmount.Equal(mustDecimal(t, "1000")), "investment amount with checking leg present: %s", bd.InvestmentAmount)
}

func TestComputeBreakdowns_RollupCreatesAncestorGroups(t *testing.T) {
	loadTestConfig(t)
	db := setupTestDB(t)

	ps := createPostings(t, db, []posting.Posting{
		// IsStockSplit treats a transaction whose only posting is a
		// non-currency commodity as a split, so every buy needs its
		// balancing currency leg like a real double-entry transaction.
		{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "1000"), Quantity: mustDecimal(t, "10"), MarketAmount: mustDecimal(t, "1200")},
		{TransactionID: "t1", Account: "Assets:Checking", Commodity: "INR", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "-1000"), Quantity: mustDecimal(t, "-1000"), MarketAmount: mustDecimal(t, "-1000")},
		{TransactionID: "t2", Account: "Assets:Equity:GOOG", Commodity: "GOOG", Date: mustDate(t, "2023-01-02"), Amount: mustDecimal(t, "2000"), Quantity: mustDecimal(t, "5"), MarketAmount: mustDecimal(t, "2500")},
		{TransactionID: "t2", Account: "Assets:Checking", Commodity: "INR", Date: mustDate(t, "2023-01-02"), Amount: mustDecimal(t, "-2000"), Quantity: mustDecimal(t, "-2000"), MarketAmount: mustDecimal(t, "-2000")},
	})

	breakdowns := ComputeBreakdowns(db, ps, true)

	// leaf groups present
	_, ok := breakdowns["Assets:Equity:AAPL"]
	assert.True(t, ok, "expected leaf group Assets:Equity:AAPL")
	_, ok = breakdowns["Assets:Equity:GOOG"]
	assert.True(t, ok, "expected leaf group Assets:Equity:GOOG")

	// rollup ancestor groups aggregate both leaves
	parent, ok := breakdowns["Assets:Equity"]
	assert.True(t, ok, "expected rollup ancestor group Assets:Equity")
	assert.True(t, parent.MarketAmount.Equal(mustDecimal(t, "3700")), "parent market amount: %s", parent.MarketAmount)
	assert.True(t, parent.InvestmentAmount.Equal(mustDecimal(t, "3000")), "parent investment amount: %s", parent.InvestmentAmount)
	// non-leaf groups don't compute balance units
	assert.True(t, parent.BalanceUnits.Equal(decimal.Zero), "non-leaf balance units should be zero: %s", parent.BalanceUnits)

	root, ok := breakdowns["Assets"]
	assert.True(t, ok, "expected root rollup group Assets")
	// root also picks up the two balancing Assets:Checking legs (-1000, -2000)
	assert.True(t, root.MarketAmount.Equal(mustDecimal(t, "700")), "root market amount: %s", root.MarketAmount)
}

func TestComputeBreakdowns_CapitalGainsRedirectedToSourceAccount(t *testing.T) {
	loadTestConfig(t)
	db := setupTestDB(t)

	ps := createPostings(t, db, []posting.Posting{
		{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "1000"), Quantity: mustDecimal(t, "10"), MarketAmount: mustDecimal(t, "1000")},
		{TransactionID: "t1", Account: "Assets:Checking", Commodity: "INR", Date: mustDate(t, "2023-01-01"), Amount: mustDecimal(t, "-1000"), Quantity: mustDecimal(t, "-1000"), MarketAmount: mustDecimal(t, "-1000")},
		// a capital gains posting for the same underlying commodity - should
		// be excluded from InvestmentAmount/WithdrawalAmount/MarketAmount
		// but still grouped into the Assets:Equity:AAPL breakdown. Balanced
		// against Assets:Checking (currency, so IsStockSplit short-circuits
		// on it regardless of transaction shape) rather than another AAPL
		// leg, to keep the AAPL group's postings unambiguous.
		{TransactionID: "t2", Account: "Income:CapitalGains:Equity:AAPL", Commodity: "INR", Date: mustDate(t, "2023-02-01"), Amount: mustDecimal(t, "-50"), Quantity: mustDecimal(t, "-50"), MarketAmount: mustDecimal(t, "-50")},
		{TransactionID: "t2", Account: "Assets:Checking", Commodity: "INR", Date: mustDate(t, "2023-02-01"), Amount: mustDecimal(t, "50"), Quantity: mustDecimal(t, "50"), MarketAmount: mustDecimal(t, "50")},
	})

	breakdowns := ComputeBreakdowns(db, ps, false)
	bd, ok := breakdowns["Assets:Equity:AAPL"]
	assert.True(t, ok, "expected group Assets:Equity:AAPL")
	// capital gains leg must not leak into these sums
	assert.True(t, bd.InvestmentAmount.Equal(mustDecimal(t, "1000")), "investment amount: %s", bd.InvestmentAmount)
	assert.True(t, bd.MarketAmount.Equal(mustDecimal(t, "1000")), "market amount excludes capital gains leg: %s", bd.MarketAmount)

	// the capital gains account itself is not a separate breakdown group
	_, ok = breakdowns["Income:CapitalGains:Equity:AAPL"]
	assert.False(t, ok, "capital gains account should not form its own group")
}
