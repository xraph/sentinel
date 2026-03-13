package dashboard

import (
	"github.com/xraph/forge/extensions/dashboard/contributor"

	"github.com/xraph/sentinel/dashboard/components"
)

// NewManifest builds a contributor.Manifest for the sentinel dashboard.
func NewManifest() *contributor.Manifest {
	return &contributor.Manifest{
		Name:        "sentinel",
		DisplayName: "Sentinel",
		Icon:        "shield-check",
		Version:     "0.1.0",
		Layout:      "extension",
		ShowSidebar: boolPtr(true),
		TopbarConfig: &contributor.TopbarConfig{
			Title:       "Sentinel",
			LogoIcon:    "shield-check",
			AccentColor: "#10b981",
			ShowSearch:  true,
			Actions: []contributor.TopbarAction{
				{Label: "API Docs", Icon: "file-text", Href: "/docs", Variant: "ghost"},
			},
		},
		SidebarFooterContent: components.FooterAPIDocsLink("/docs"),
		Nav:                  baseNav(),
		Widgets:              baseWidgets(),
		Settings:             baseSettings(),
		Capabilities: []string{
			"searchable",
		},
	}
}

func baseNav() []contributor.NavItem {
	return []contributor.NavItem{
		{Label: "Overview", Path: "/", Icon: "layout-dashboard", Group: "Sentinel", Priority: 0},
		{Label: "Suites", Path: "/suites", Icon: "folders", Group: "Evaluation", Priority: 1},
		{Label: "Runs", Path: "/runs", Icon: "play", Group: "Evaluation", Priority: 2},
		{Label: "Baselines", Path: "/baselines", Icon: "git-branch", Group: "Evaluation", Priority: 3},
		{Label: "Scorers", Path: "/scorers", Icon: "target", Group: "Reference", Priority: 4},
	}
}

func baseWidgets() []contributor.WidgetDescriptor {
	return []contributor.WidgetDescriptor{
		{
			ID:          "sentinel-stats",
			Title:       "Evaluation Stats",
			Description: "Sentinel entity counts and aggregate pass rate",
			Size:        "md",
			RefreshSec:  60,
			Group:       "Sentinel",
		},
		{
			ID:          "sentinel-recent-runs",
			Title:       "Recent Runs",
			Description: "Recent evaluation run results",
			Size:        "lg",
			RefreshSec:  15,
			Group:       "Sentinel",
		},
	}
}

func baseSettings() []contributor.SettingsDescriptor {
	return []contributor.SettingsDescriptor{
		{
			ID:          "sentinel-config",
			Title:       "Engine Settings",
			Description: "Configure Sentinel engine behavior",
			Group:       "Sentinel",
			Icon:        "shield-check",
		},
	}
}

func boolPtr(b bool) *bool { return &b }
