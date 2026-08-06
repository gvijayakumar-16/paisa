package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/stretchr/testify/assert"
)

func writeFile(t *testing.T, dir string, name string, content string) string {
	path := filepath.Join(dir, name)
	assert.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestHasPeriodicTransactions_NoPeriodicDirectives(t *testing.T) {
	dir := t.TempDir()
	root := writeFile(t, dir, "main.ledger", "2023-01-01 * Salary\n    Assets:Checking  1000 INR\n    Income:Salary\n")
	assert.False(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_DirectlyInRoot(t *testing.T) {
	dir := t.TempDir()
	root := writeFile(t, dir, "main.ledger", "~ Monthly\n    Expenses:Rent  1000 INR\n    Assets:Checking\n")
	assert.True(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_IndentedPeriodicDirective(t *testing.T) {
	dir := t.TempDir()
	root := writeFile(t, dir, "main.ledger", "  ~ Monthly\n    Expenses:Rent  1000 INR\n")
	assert.True(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_ViaSingleInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "budget.ledger", "~ Monthly\n    Expenses:Rent  1000 INR\n")
	root := writeFile(t, dir, "main.ledger", "include budget.ledger\n2023-01-01 * Salary\n    Assets:Checking  1000 INR\n    Income:Salary\n")
	assert.True(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_ViaGlobInclude(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(dir, "accounts"), 0755))
	writeFile(t, dir, "accounts/a.ledger", "2023-01-01 * Salary\n    Assets:Checking  1000 INR\n    Income:Salary\n")
	writeFile(t, dir, "accounts/b.ledger", "~ Monthly\n    Expenses:Rent  1000 INR\n")
	root := writeFile(t, dir, "main.ledger", "include accounts/*.ledger\n")
	assert.True(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_GlobIncludeWithoutPeriodic(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(dir, "accounts"), 0755))
	writeFile(t, dir, "accounts/a.ledger", "2023-01-01 * Salary\n    Assets:Checking  1000 INR\n    Income:Salary\n")
	writeFile(t, dir, "accounts/b.ledger", "2023-01-02 * Rent\n    Expenses:Rent  1000 INR\n    Assets:Checking\n")
	root := writeFile(t, dir, "main.ledger", "include accounts/*.ledger\n")
	assert.False(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_NestedInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "leaf.ledger", "~ Monthly\n    Expenses:Rent  1000 INR\n")
	writeFile(t, dir, "mid.ledger", "include leaf.ledger\n")
	root := writeFile(t, dir, "main.ledger", "include mid.ledger\n")
	assert.True(t, hasPeriodicTransactions(root))
}

func TestHasPeriodicTransactions_MissingFile(t *testing.T) {
	assert.False(t, hasPeriodicTransactions("/nonexistent/path/main.ledger"))
}

func TestHasPeriodicTransactions_CircularInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ledger", "include b.ledger\n")
	root := writeFile(t, dir, "b.ledger", "include a.ledger\n")
	// must terminate rather than infinite-loop, and correctly report false
	assert.False(t, hasPeriodicTransactions(root))
}

func assertPriceEqual(t *testing.T, actual price.Price, date string, commodityName string, value float64) {
	assert.Equal(t, commodityName, actual.CommodityName, "they should be equal")
	assert.Equal(t, date, actual.Date.Format("2006/01/02"), "they should be equal")
	assert.Equal(t, value, actual.Value.InexactFloat64(), "they should be equal")
}

func TestParseLegerPrices(t *testing.T) {
	parsedPrices, _ := parseLedgerPrices("P 2023/05/01 00:00:00 USD 0.9 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "USD", 0.9)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR $1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR $-1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", -1.1)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 EUR ₹70\n", "₹")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 70)

	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 USD 0.9 EUR\n", "INR")
	assert.Len(t, parsedPrices, 0)
	parsedPrices, _ = parseLedgerPrices("P 2023/05/01 00:00:00 USD $0.9\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseLedgerPrices("P 2022/01/29 00:50:00 UAH 0.026 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2022/01/29", "UAH", 0.026)
}

func TestParseHLegerPrices(t *testing.T) {
	parsedPrices, _ := parseHLedgerPrices("P 2023-05-01 USD 0.9 EUR\n", "EUR")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "USD", 0.9)
	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR $1.1\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR USD 1.1\n", "USD")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 EUR 1.1$\n", "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices(utils.Dos2Unix("P 2023-05-01 EUR 1.1$\r\n"), "$")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "EUR", 1.1)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 \"AAPL0\" \"USD0\" 45.5\n", "USD0")
	assertPriceEqual(t, parsedPrices[0], "2023/05/01", "AAPL0", 45.5)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD 0.9 EUR\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD $0.9\n", "INR")
	assert.Len(t, parsedPrices, 0)

	parsedPrices, _ = parseHLedgerPrices("P 2023-05-01 USD $0.9\r\n", "INR")
	assert.Len(t, parsedPrices, 0)
}

func TestParseAmount(t *testing.T) {
	commodity, amount, _ := parseAmount("0.9 USD")
	assert.Equal(t, "USD", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("$0.9")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("0.9$")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, 0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("$-0.9")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, -0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("-0.9$")
	assert.Equal(t, "$", commodity)
	assert.Equal(t, -0.9, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100,000 EUR")
	assert.Equal(t, "EUR", commodity)
	assert.Equal(t, 100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100,000.00 \"EUR0-0\"")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, 100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("-100,000.00 \"EUR0-0\"")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, -100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("\"EUR0-0\" -100,000.00")
	assert.Equal(t, "EUR0-0", commodity)
	assert.Equal(t, -100000.0, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("INR 70.0099")
	assert.Equal(t, "INR", commodity)
	assert.Equal(t, 70.0099, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("1E-8 BTC")
	assert.Equal(t, "BTC", commodity)
	assert.Equal(t, 1e-08, amount.InexactFloat64())

	commodity, amount, _ = parseAmount("100E-8    BTC")
	assert.Equal(t, "BTC", commodity)
	assert.Equal(t, 1e-06, amount.InexactFloat64())
}
