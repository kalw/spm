package versions

import (
	"reflect"
	"testing"
)

func TestMergeSortsAndDedups(t *testing.T) {
	list := []string{"1.0.0", "1.10.0", "1.2.0"}
	got, err := Merge(list, "1.3.0", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1.0.0", "1.2.0", "1.3.0", "1.10.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge = %v, want %v", got, want)
	}
}

func TestMergeRejectsDuplicate(t *testing.T) {
	if _, err := Merge([]string{"1.0.0"}, "1.0.0", false); err == nil {
		t.Fatal("duplicate version should error without force")
	}
}

func TestMergeForceAllowsDuplicate(t *testing.T) {
	got, err := Merge([]string{"1.0.0"}, "1.0.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"1.0.0"}) {
		t.Fatalf("Merge force = %v", got)
	}
}

func TestMergeRejectsNonSemver(t *testing.T) {
	if _, err := Merge(nil, "1.2", false); err == nil {
		t.Fatal("non-semver version should error")
	}
}

func TestParseEmpty(t *testing.T) {
	list, err := Parse(nil)
	if err != nil || list != nil {
		t.Fatalf("Parse(nil) = %v, %v", list, err)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	b, err := Encode([]string{"1.0.0", "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(list, []string{"1.0.0", "2.0.0"}) {
		t.Fatalf("round trip = %v", list)
	}
}
