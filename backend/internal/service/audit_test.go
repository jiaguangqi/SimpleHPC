package service

import "testing"

func TestNormalizeAuditQueryBoundsPagination(t *testing.T) {
	got := normalizeAuditQuery(AuditQuery{Page: -1, PageSize: 999, Result: " SUCCESS "})
	if got.Page != 1 || got.PageSize != 100 || got.Result != "success" || got.Offset != 0 {
		t.Fatalf("unexpected query: %#v", got)
	}
}

func TestNormalizePlatformConfigAppliesDefaults(t *testing.T) {
	got, err := NormalizePlatformConfig(PlatformConfig{Name: "  HPC Center  "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "HPC Center" || got.Language != "zh-CN" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestPlatformLanguageIsCurrentlyChineseOnly(t *testing.T) {
	if _, err := NormalizePlatformConfig(PlatformConfig{Name: "HPC", Language: "en-US"}); err == nil {
		t.Fatal("English must remain unavailable until translations are implemented")
	}
}

func TestValidatePlatformImages(t *testing.T) {
	if err := ValidatePlatformImage("logo", 2*1024*1024, 512, 256, "jpeg"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePlatformImage("logo", 2*1024*1024+1, 512, 256, "jpeg"); err == nil {
		t.Fatal("oversized logo must fail")
	}
	if err := ValidatePlatformImage("login-image", 8*1024*1024, 1919, 1080, "png"); err == nil {
		t.Fatal("undersized login image must fail")
	}
	if err := ValidatePlatformImage("login-image", 1024, 1920, 1080, "gif"); err == nil {
		t.Fatal("unsupported format must fail")
	}
}
