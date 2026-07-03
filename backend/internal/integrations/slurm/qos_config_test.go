package slurm

import "testing"

func TestValidateQOS(t *testing.T) {
	if err := validateQOS(QOS{Name: "gpu-high", MaxJobsPU: "10", MaxWall: "48:00:00"}); err != nil {
		t.Fatalf("valid QOS rejected: %v", err)
	}
	if err := validateQOS(QOS{Name: "bad;name"}); err == nil {
		t.Fatal("unsafe QOS name accepted")
	}
	if err := validateQOS(QOS{Name: "normal", MaxJobsPU: "ten"}); err == nil {
		t.Fatal("non-numeric job limit accepted")
	}
}

func TestUniqueValidNames(t *testing.T) {
	items := uniqueValidNames([]string{"user02", "user01"})
	if len(items) != 2 || items[0] != "user01" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
