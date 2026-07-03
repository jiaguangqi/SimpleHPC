package service

import "testing"

func TestMenuCatalogCarriesRouteAndResourceMetadata(t *testing.T) {
	for _, item := range DefaultMenuCatalog() {
		if item.Type != "page" {
			continue
		}
		if item.RoutePermission == "" {
			t.Fatalf("menu %s lacks route permission", item.Code)
		}
		if item.Resource == "" {
			t.Fatalf("menu %s lacks resource metadata", item.Code)
		}
	}
}
