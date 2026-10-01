package domain

import (
	"maps"

	"github.com/google/uuid"

	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

// Haggle is a Character's haggling at the open Shop: its roll while it is out, then the price adjustment.
type Haggle struct {
	RollID *RollID
	Adjust *int
}

// OpenShop is the Shop open in a Session: the Shop with its Stock, where it is, who keeps it, what its
// items are, and each Character's haggling there.
type OpenShop struct {
	Shop       prep.Shop
	Settlement string
	Owner      string
	Items      map[string]prep.ItemPrice
	Haggles    map[uuid.UUID]Haggle
}

// Clone copies the open Shop so a change never touches the committed state.
func (o *OpenShop) Clone() *OpenShop {
	c := *o
	c.Shop.Stock = append([]prep.StockItem(nil), o.Shop.Stock...)
	c.Items, c.Haggles = maps.Clone(o.Items), maps.Clone(o.Haggles)
	return &c
}

// Trade is a purchase or a sale: what moved between the Shop and a Character, the Character's purse
// afterwards, and how many the Shop has left.
type Trade struct {
	Container ContainerID
	Label     string
	Item      string
	Count     int
	Purse     map[string]int
	Carried   int
	StockLeft int
	// StockPrice is the asking price of what the Shop has left of it; PriceCP what changed hands.
	StockPrice int
	PriceCP    int
	Shop       string
}

// Shop action kinds in the Action Log.
const (
	ActionShopOpened    = "shop_opened"
	ActionShopClosed    = "shop_closed"
	ActionItemBought    = "item_bought"
	ActionItemSold      = "item_sold"
	ActionHaggleStarted = "haggle_started"
	ActionHaggled       = "haggled"
	ActionStockRolled   = "stock_rolled"
)
