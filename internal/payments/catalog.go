package payments

import (
	"context"
	"fmt"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
)

// CatalogClient manages Products/Prices on a merchant Stripe account (org secret key).
type CatalogClient interface {
	EnsureProductPrice(ctx context.Context, in EnsureProductPriceInput) (ProductPriceRefs, error)
	UpdateProductName(ctx context.Context, productRef, name string) error
	ListCatalog(ctx context.Context) ([]CatalogItem, error)
}

type EnsureProductPriceInput struct {
	Name               string
	Interval           string // month|year
	Currency           string
	UnitAmount         int64
	ExistingProductRef string
	ExistingPriceRef   string
	Metadata           map[string]string
}

type ProductPriceRefs struct {
	ProductRef string
	PriceRef   string
}

type CatalogItem struct {
	ProductRef  string
	ProductName string
	PriceRef    string
	Interval    string
	Currency    string
	UnitAmount  int64
	Active      bool
}

// StripeCatalog talks to Stripe Products/Prices using a per-org secret key.
type StripeCatalog struct {
	secretKey string
}

func NewStripeCatalog(secretKey string) (*StripeCatalog, error) {
	key := strings.TrimSpace(secretKey)
	if key == "" {
		return nil, apperr.Validation("Stripe secret key is required")
	}
	if !strings.HasPrefix(key, "sk_") {
		return nil, apperr.Validation("Stripe secret key must start with sk_")
	}
	return &StripeCatalog{secretKey: key}, nil
}

func (c *StripeCatalog) EnsureProductPrice(ctx context.Context, in EnsureProductPriceInput) (ProductPriceRefs, error) {
	stripe.Key = c.secretKey
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ProductPriceRefs{}, apperr.Validation("product name is required")
	}
	interval := strings.TrimSpace(in.Interval)
	switch interval {
	case "month", "year":
	default:
		return ProductPriceRefs{}, apperr.Validation("interval must be month or year")
	}
	currency := strings.ToLower(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "usd"
	}
	if in.UnitAmount < 0 {
		return ProductPriceRefs{}, apperr.Validation("unit_amount must be >= 0")
	}

	productRef := strings.TrimSpace(in.ExistingProductRef)
	if productRef == "" {
		params := &stripe.ProductParams{
			Name: stripe.String(name),
		}
		params.Context = ctx
		for k, v := range in.Metadata {
			params.AddMetadata(k, v)
		}
		prod, err := product.New(params)
		if err != nil {
			return ProductPriceRefs{}, fmt.Errorf("stripe create product: %w", err)
		}
		productRef = prod.ID
	} else if err := c.UpdateProductName(ctx, productRef, name); err != nil {
		return ProductPriceRefs{}, err
	}

	priceRef := strings.TrimSpace(in.ExistingPriceRef)
	reuse := false
	if priceRef != "" {
		existing, err := price.Get(priceRef, &stripe.PriceParams{Params: stripe.Params{Context: ctx}})
		if err == nil && existing != nil &&
			existing.Active &&
			existing.UnitAmount == in.UnitAmount &&
			strings.EqualFold(string(existing.Currency), currency) &&
			existing.Recurring != nil &&
			string(existing.Recurring.Interval) == interval {
			reuse = true
		}
	}
	if !reuse {
		params := &stripe.PriceParams{
			Product:    stripe.String(productRef),
			Currency:   stripe.String(currency),
			UnitAmount: stripe.Int64(in.UnitAmount),
			Recurring: &stripe.PriceRecurringParams{
				Interval: stripe.String(interval),
			},
		}
		params.Context = ctx
		for k, v := range in.Metadata {
			params.AddMetadata(k, v)
		}
		p, err := price.New(params)
		if err != nil {
			return ProductPriceRefs{}, fmt.Errorf("stripe create price: %w", err)
		}
		priceRef = p.ID
	}

	return ProductPriceRefs{ProductRef: productRef, PriceRef: priceRef}, nil
}

func (c *StripeCatalog) UpdateProductName(ctx context.Context, productRef, name string) error {
	stripe.Key = c.secretKey
	productRef = strings.TrimSpace(productRef)
	name = strings.TrimSpace(name)
	if productRef == "" || name == "" {
		return nil
	}
	params := &stripe.ProductParams{Name: stripe.String(name)}
	params.Context = ctx
	_, err := product.Update(productRef, params)
	if err != nil {
		return fmt.Errorf("stripe update product: %w", err)
	}
	return nil
}

func (c *StripeCatalog) ListCatalog(ctx context.Context) ([]CatalogItem, error) {
	stripe.Key = c.secretKey
	params := &stripe.PriceListParams{
		Active: stripe.Bool(true),
		Type:   stripe.String(string(stripe.PriceTypeRecurring)),
		Expand: []*string{stripe.String("data.product")},
	}
	params.Context = ctx
	params.Limit = stripe.Int64(100)

	out := make([]CatalogItem, 0)
	iter := price.List(params)
	for iter.Next() {
		p := iter.Price()
		if p == nil || p.Recurring == nil {
			continue
		}
		interval := string(p.Recurring.Interval)
		if interval != "month" && interval != "year" {
			continue
		}
		item := CatalogItem{
			PriceRef:   p.ID,
			Interval:   interval,
			Currency:   string(p.Currency),
			UnitAmount: p.UnitAmount,
			Active:     p.Active,
		}
		if p.Product != nil {
			item.ProductRef = p.Product.ID
			item.ProductName = p.Product.Name
			if !p.Product.Active {
				continue
			}
		}
		out = append(out, item)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("stripe list prices: %w", err)
	}
	return out, nil
}

// NoopCatalog is used in tests / when Stripe is not connected.
type NoopCatalog struct{}

func NewNoopCatalog() *NoopCatalog { return &NoopCatalog{} }

func (n *NoopCatalog) EnsureProductPrice(_ context.Context, in EnsureProductPriceInput) (ProductPriceRefs, error) {
	prod := strings.TrimSpace(in.ExistingProductRef)
	if prod == "" {
		prod = "prod_noop_" + slugRef(in.Name)
	}
	priceRef := strings.TrimSpace(in.ExistingPriceRef)
	if priceRef == "" {
		priceRef = "price_noop_" + slugRef(in.Name) + "_" + in.Interval
	}
	return ProductPriceRefs{ProductRef: prod, PriceRef: priceRef}, nil
}

func (n *NoopCatalog) UpdateProductName(context.Context, string, string) error { return nil }

func (n *NoopCatalog) ListCatalog(context.Context) ([]CatalogItem, error) {
	return []CatalogItem{}, nil
}

func slugRef(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "item"
	}
	return out
}
