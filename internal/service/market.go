package service

import (
	"sort"
	"sync"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/google/btree"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type priceCache struct {
	sync.Once
	pricesTree        map[string]*btree.BTree
	postingPricesTree map[string]*btree.BTree
}

var pcache priceCache

func loadPriceCache(db *gorm.DB) {
	var prices []price.Price
	result := db.Where("commodity_type != ?", config.Unknown).Find(&prices)
	if result.Error != nil {
		log.Fatal(result.Error)
	}
	pcache.pricesTree = make(map[string]*btree.BTree)
	pcache.postingPricesTree = make(map[string]*btree.BTree)

	for _, price := range prices {
		if pcache.pricesTree[price.CommodityName] == nil {
			pcache.pricesTree[price.CommodityName] = btree.New(2)
		}

		pcache.pricesTree[price.CommodityName].ReplaceOrInsert(price)
	}

	var postings []posting.Posting
	result = db.Find(&postings)
	if result.Error != nil {
		log.Fatal(result.Error)
	}

	postingsByCommodity := lo.GroupBy(postings, func(p posting.Posting) string { return p.Commodity })

	var nonCurrencyCommodities []string
	for commodityName, postings := range postingsByCommodity {
		if !utils.IsCurrency(postings[0].Commodity) {
			nonCurrencyCommodities = append(nonCurrencyCommodities, commodityName)
		}
	}

	// One query for every distinct non-currency commodity instead of one
	// query per commodity (previously N+1 - on a portfolio with 100+
	// distinct holdings, that's 100+ round-trips every time this cache is
	// rebuilt after a sync).
	var unknownPrices []price.Price
	if len(nonCurrencyCommodities) > 0 {
		result = db.Where("commodity_type = ? and commodity_name in ?", config.Unknown, nonCurrencyCommodities).Find(&unknownPrices)
		if result.Error != nil {
			log.Fatal(result.Error)
		}
	}
	unknownPricesByCommodity := lo.GroupBy(unknownPrices, func(p price.Price) string { return p.CommodityName })

	for _, commodityName := range nonCurrencyCommodities {
		postingPricesTree := btree.New(2)
		for _, price := range unknownPricesByCommodity[commodityName] {
			postingPricesTree.ReplaceOrInsert(price)
		}
		pcache.postingPricesTree[commodityName] = postingPricesTree

		if pcache.pricesTree[commodityName] == nil {
			pcache.pricesTree[commodityName] = postingPricesTree
		}
	}
}

func ClearPriceCache() {
	pcache = priceCache{}
}

func GetUnitPrice(db *gorm.DB, commodity string, date time.Time) price.Price {
	pcache.Do(func() { loadPriceCache(db) })

	pt := pcache.pricesTree[commodity]
	if pt == nil {
		log.Fatal("Price not found ", commodity)
	}

	pc := utils.BTreeDescendFirstLessOrEqual(pt, price.Price{Date: date})
	if !pc.Value.Equal(decimal.Zero) {
		return pc
	}

	pt = pcache.postingPricesTree[commodity]
	if pt == nil {
		log.Fatal("Price not found ", commodity)
	}
	return utils.BTreeDescendFirstLessOrEqual(pt, price.Price{Date: date})

}

func GetAllPrices(db *gorm.DB, commodity string) []price.Price {
	pcache.Do(func() { loadPriceCache(db) })

	pt := pcache.postingPricesTree[commodity]
	if pt == nil {
		log.Fatal("Price not found ", commodity)
	}

	pmap := make(map[string]price.Price)

	for _, price := range utils.BTreeToSlice[price.Price](pt) {
		pmap[price.Date.String()] = price
	}

	pt = pcache.pricesTree[commodity]
	if pt == nil {
		log.Fatal("Price not found ", commodity)
	}

	for _, price := range utils.BTreeToSlice[price.Price](pt) {
		pmap[price.Date.String()] = price
	}

	prices := []price.Price{}
	keys := lo.Keys(pmap)
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		prices = append(prices, pmap[key])
	}

	return prices
}

func GetMarketPrice(db *gorm.DB, p posting.Posting, date time.Time) decimal.Decimal {
	if utils.IsCurrency(p.Commodity) {
		return p.Amount
	}

	pc := GetUnitPrice(db, p.Commodity, date)
	if !pc.Value.Equal(decimal.Zero) {
		return p.Quantity.Mul(pc.Value)
	}

	return p.Amount
}

func GetPrice(db *gorm.DB, commodity string, quantity decimal.Decimal, date time.Time) decimal.Decimal {
	if utils.IsCurrency(commodity) {
		return quantity
	}

	pc := GetUnitPrice(db, commodity, date)
	if !pc.Value.Equal(decimal.Zero) {
		return quantity.Mul(pc.Value)
	}

	return quantity
}

func PopulateMarketPrice(db *gorm.DB, ps []posting.Posting) []posting.Posting {
	date := utils.EndOfToday()
	return lo.Map(ps, func(p posting.Posting, _ int) posting.Posting {
		p.MarketAmount = GetMarketPrice(db, p, date)
		return p
	})
}
