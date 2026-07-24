package resources

import (
	"testing"
)

func TestExpandTriggersIncludesAttributesAndOtherResources(t *testing.T) {
	triggers := ExpandTriggers(Default(), map[string][]AttributeKey{
		AppUser: {
			{Key: "country", Label: "Country"},
			{Key: "plan_tier", Label: "Plan tier"},
		},
	})
	byID := map[string]TriggerInfo{}
	for _, tr := range triggers {
		byID[tr.ID] = tr
	}
	for _, want := range []string{
		"app_user.created",
		"app_user.updated",
		"app_user.deleted",
		"app_user.attribute.country",
		"app_user.attribute.plan_tier",
		"membership.created",
		"membership.updated",
		"subscription.created",
		"subscription.updated",
		"schedule.hourly",
		"schedule.daily",
		"schedule.weekly",
		"webhook.received",
	} {
		if _, ok := byID[want]; !ok {
			t.Fatalf("missing trigger %s in %#v", want, byID)
		}
	}
	if byID["schedule.daily"].Kind != string(KindSchedule) {
		t.Fatalf("schedule kind=%s", byID["schedule.daily"].Kind)
	}
	if byID["webhook.received"].Kind != string(KindWebhook) {
		t.Fatalf("webhook kind=%s", byID["webhook.received"].Kind)
	}
}

func TestParseTrigger(t *testing.T) {
	tr, err := ParseTrigger("app_user.attribute.country")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Kind != KindAttribute || tr.Event != "country" || tr.Resource != AppUser {
		t.Fatalf("%+v", tr)
	}
	if _, err := ParseTrigger("nope.created"); err == nil {
		t.Fatal("want error for unknown resource")
	}
	if _, err := ParseTrigger("membership.attribute.x"); err == nil {
		t.Fatal("membership does not support attributes")
	}
}

func TestChangedAttributeKeys(t *testing.T) {
	keys := ChangedAttributeKeys(
		map[string]any{"country": "AU", "seats": 1},
		map[string]any{"country": "NZ", "plan_tier": "pro"},
	)
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	for _, want := range []string{"country", "seats", "plan_tier"} {
		if !set[want] {
			t.Fatalf("want %s in %v", want, keys)
		}
	}
}
