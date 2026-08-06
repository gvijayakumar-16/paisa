package service

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupMarketTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	assert.NoError(t, err)
	model.AutoMigrate(db)

	err = config.LoadConfig([]byte("journal_path: main.ledger\ndb_path: main.db\ndefault_currency: INR\n"), "")
	assert.NoError(t, err)

	ClearPriceCache()
	t.Cleanup(ClearPriceCache)

	return db
}

func mustDate2(t *testing.T, s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	assert.NoError(t, err)
	return d
}

func TestGetUnitPrice_UsesPostingDerivedPriceForUnknownCommodities(t *testing.T) {
	db := setupMarketTestDB(t)

	// two distinct commodities, each with their own "Unknown"-typed price
	// history recorded from postings (not a real market price feed) -
	// loadPriceCache used to issue one query per distinct commodity here
	// (an N+1), now batched into a single query.
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-01-01"), CommodityType: config.Unknown, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(100)}).Error)
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-06-01"), CommodityType: config.Unknown, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(150)}).Error)
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-01-01"), CommodityType: config.Unknown, CommodityID: "GOOG", CommodityName: "GOOG", Value: decimal.NewFromInt(2000)}).Error)

	assert.NoError(t, db.Create(&posting.Posting{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate2(t, "2023-01-01"), Amount: decimal.NewFromInt(1000), Quantity: decimal.NewFromInt(10)}).Error)
	assert.NoError(t, db.Create(&posting.Posting{TransactionID: "t2", Account: "Assets:Equity:GOOG", Commodity: "GOOG", Date: mustDate2(t, "2023-01-01"), Amount: decimal.NewFromInt(2000), Quantity: decimal.NewFromInt(1)}).Error)
	// a currency posting - must not be treated as a commodity needing an
	// Unknown-price lookup at all.
	assert.NoError(t, db.Create(&posting.Posting{TransactionID: "t1", Account: "Assets:Checking", Commodity: "INR", Date: mustDate2(t, "2023-01-01"), Amount: decimal.NewFromInt(-1000), Quantity: decimal.NewFromInt(-1000)}).Error)

	aapl := GetUnitPrice(db, "AAPL", mustDate2(t, "2023-12-31"))
	assert.True(t, aapl.Value.Equal(decimal.NewFromInt(150)), "AAPL latest price: %s", aapl.Value)

	aaplEarly := GetUnitPrice(db, "AAPL", mustDate2(t, "2023-03-01"))
	assert.True(t, aaplEarly.Value.Equal(decimal.NewFromInt(100)), "AAPL price as of March: %s", aaplEarly.Value)

	goog := GetUnitPrice(db, "GOOG", mustDate2(t, "2023-12-31"))
	assert.True(t, goog.Value.Equal(decimal.NewFromInt(2000)), "GOOG latest price: %s", goog.Value)
}

func TestGetUnitPrice_PrefersRealPriceOverPostingDerivedPrice(t *testing.T) {
	db := setupMarketTestDB(t)

	// a real (non-Unknown) price takes precedence over the Unknown/
	// posting-derived one for the same commodity/date.
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-01-01"), CommodityType: config.MutualFund, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(999)}).Error)
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-01-01"), CommodityType: config.Unknown, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(100)}).Error)
	assert.NoError(t, db.Create(&posting.Posting{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate2(t, "2023-01-01"), Amount: decimal.NewFromInt(1000), Quantity: decimal.NewFromInt(10)}).Error)

	aapl := GetUnitPrice(db, "AAPL", mustDate2(t, "2023-12-31"))
	assert.True(t, aapl.Value.Equal(decimal.NewFromInt(999)), "expected the real price to win: %s", aapl.Value)
}

func TestGetAllPrices_ReturnsPostingDerivedPricesForCommodity(t *testing.T) {
	db := setupMarketTestDB(t)

	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-01-01"), CommodityType: config.Unknown, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(100)}).Error)
	assert.NoError(t, db.Create(&price.Price{Date: mustDate2(t, "2023-06-01"), CommodityType: config.Unknown, CommodityID: "AAPL", CommodityName: "AAPL", Value: decimal.NewFromInt(150)}).Error)
	assert.NoError(t, db.Create(&posting.Posting{TransactionID: "t1", Account: "Assets:Equity:AAPL", Commodity: "AAPL", Date: mustDate2(t, "2023-01-01"), Amount: decimal.NewFromInt(1000), Quantity: decimal.NewFromInt(10)}).Error)

	prices := GetAllPrices(db, "AAPL")
	assert.Len(t, prices, 2)
	// newest first
	assert.True(t, prices[0].Value.Equal(decimal.NewFromInt(150)), "prices[0]: %s", prices[0].Value)
	assert.True(t, prices[1].Value.Equal(decimal.NewFromInt(100)), "prices[1]: %s", prices[1].Value)
}
