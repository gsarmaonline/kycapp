package resources

import "testing"

func TestAvailableSubjectsAppUser(t *testing.T) {
	got, err := AvailableSubjects("app_user.created")
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, s := range got {
		set[s] = true
	}
	if !set[SubjectAppUser] {
		t.Fatalf("want app_user in %v", got)
	}
}

func TestAvailableSubjectsMembershipIncludesUser(t *testing.T) {
	got, err := AvailableSubjectSet("membership.created")
	if err != nil {
		t.Fatal(err)
	}
	if !got[SubjectMembership] || !got[SubjectUser] {
		t.Fatalf("want membership+user, got %v", got)
	}
	if got[SubjectAppUser] {
		t.Fatal("membership must not provide app_user without an explicit relation")
	}
}

func TestMissingSubjectsSendEmailOnSubscription(t *testing.T) {
	missing, err := MissingSubjects("subscription.created", []string{SubjectAppUser})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != SubjectAppUser {
		t.Fatalf("want missing app_user, got %v", missing)
	}
	missing, err = MissingSubjects("app_user.attribute.country", []string{SubjectAppUser})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("app_user trigger should satisfy app_user: %v", missing)
	}
}
