package payments

import (
	"context"
	"testing"
)

func TestNoopCatalogEnsureProductPrice(t *testing.T) {
	c := NewNoopCatalog()
	refs, err := c.EnsureProductPrice(context.Background(), EnsureProductPriceInput{
		Name:       "Pro Plan",
		Interval:   "month",
		Currency:   "usd",
		UnitAmount: 2900,
	})
	if err != nil {
		t.Fatal(err)
	}
	if refs.ProductRef == "" || refs.PriceRef == "" {
		t.Fatalf("expected noop refs: %#v", refs)
	}
	again, err := c.EnsureProductPrice(context.Background(), EnsureProductPriceInput{
		Name:               "Pro Plan",
		Interval:           "month",
		Currency:           "usd",
		UnitAmount:         2900,
		ExistingProductRef: refs.ProductRef,
		ExistingPriceRef:   refs.PriceRef,
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.ProductRef != refs.ProductRef || again.PriceRef != refs.PriceRef {
		t.Fatalf("expected reuse: %#v vs %#v", again, refs)
	}
}
