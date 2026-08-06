package model

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/model/commodity"
	mutualfundModel "github.com/ananthakumaran/paisa/internal/model/mutualfund/scheme"
	npsModel "github.com/ananthakumaran/paisa/internal/model/nps/scheme"
	"github.com/ananthakumaran/paisa/internal/model/portfolio"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/scraper"
	"github.com/ananthakumaran/paisa/internal/scraper/india"
	"github.com/ananthakumaran/paisa/internal/scraper/mutualfund"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&npsModel.Scheme{})
	db.AutoMigrate(&mutualfundModel.Scheme{})
	db.AutoMigrate(&posting.Posting{})
	db.AutoMigrate(&price.Price{})
	db.AutoMigrate(&portfolio.Portfolio{})
	db.AutoMigrate(&price.Price{})
	db.AutoMigrate(&cii.CII{})
	db.AutoMigrate(&cache.Cache{})
}

func SyncJournal(db *gorm.DB) (string, error) {
	AutoMigrate(db)
	log.Info("Syncing transactions from journal")

	// ValidateFile/Prices/Parse are run sequentially, not concurrently -
	// tried overlapping ValidateFile with Prices+Parse (both spawn heavy
	// `ledger` subprocesses that re-parse the whole journal), but on a
	// real large journal they contend for the same CPU instead of
	// overlapping cleanly: both got slower running at the same time,
	// canceling out the theoretical win. Sequential measured faster.
	errors, _, err := ledger.Cli().ValidateFile(config.GetJournalPath())
	if err != nil {

		if len(errors) == 0 {
			return err.Error(), err
		}

		var message string
		for _, error := range errors {
			message += error.Message + "\n\n"
		}
		return strings.TrimRight(message, "\n"), err
	}

	prices, err := ledger.Cli().Prices(config.GetJournalPath())
	if err != nil {
		return err.Error(), err
	}

	price.UpsertAllByType(db, config.Unknown, prices)

	postings, err := ledger.Cli().Parse(config.GetJournalPath(), prices)
	if err != nil {
		return err.Error(), err
	}

	posting.UpsertAll(db, postings)

	return "", nil
}

// fetchConcurrency bounds how many price providers are hit at once.
// ponytail: fixed cap, revisit if a provider starts rate-limiting us.
const fetchConcurrency = 8

type commodityFetchResult struct {
	commodity config.Commodity
	prices    []*price.Price
	err       error
}

func SyncCommodities(db *gorm.DB) error {
	AutoMigrate(db)
	log.Info("Fetching commodities price history")
	commodities := lo.Shuffle(commodity.All())

	// Fetching is pure network IO with no shared state, so it's done
	// concurrently. Writes go through sequentially afterwards on the
	// caller's goroutine since sqlite only supports one writer at a time.
	results := make([]commodityFetchResult, len(commodities))
	sem := make(chan struct{}, fetchConcurrency)
	var wg sync.WaitGroup
	for i, c := range commodities {
		wg.Add(1)
		go func(i int, c config.Commodity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			log.Info("Fetching commodity ", c.Name)
			provider := scraper.GetProviderByCode(c.Price.Provider)
			prices, err := provider.GetPrices(c.Price.Code, c.Name)
			results[i] = commodityFetchResult{commodity: c, prices: prices, err: err}
		}(i, c)
	}
	wg.Wait()

	var errors []error
	for _, result := range results {
		if result.err != nil {
			log.Error(result.err)
			errors = append(errors, fmt.Errorf("Failed to fetch price for %s: %w", result.commodity.Name, result.err))
			continue
		}

		price.UpsertAllByTypeNameAndID(db, result.commodity.Type, result.commodity.Name, result.commodity.Price.Code, result.prices)
	}

	if len(errors) > 0 {
		var message string
		for _, error := range errors {
			message += error.Error() + "\n"
		}
		return fmt.Errorf("%s", strings.Trim(message, "\n"))
	}
	return nil
}

func SyncCII(db *gorm.DB) error {
	AutoMigrate(db)
	log.Info("Fetching taxation related info")
	ciis, err := india.GetCostInflationIndex()
	if err != nil {
		log.Error(err)
		return fmt.Errorf("Failed to fetch CII: %w", err)
	}
	cii.UpsertAll(db, ciis)
	return nil
}

func SyncPortfolios(db *gorm.DB) error {
	db.AutoMigrate(&portfolio.Portfolio{})
	log.Info("Fetching commodities portfolio")
	commodities := commodity.FindByType(config.MutualFund)
	for _, commodity := range commodities {
		if commodity.Price.Provider != "in-mfapi" {
			continue
		}

		name := commodity.Name
		log.Info("Fetching portfolio for ", name)
		portfolios, err := mutualfund.GetPortfolio(commodity.Price.Code, commodity.Name)

		if err != nil {
			log.Error(err)
			return fmt.Errorf("Failed to fetch portfolio for %s: %w", name, err)
		}

		portfolio.UpsertAll(db, commodity.Type, commodity.Price.Code, portfolios)
	}
	return nil
}
