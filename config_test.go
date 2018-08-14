package main

import "testing"

var upgrades = []struct {
	Config         Config
	ContainerScope string
	ContainerImage string
	Upgrade        bool
}{
	{
		Config: Config{
			Image: "ee01",
			Scope: "ee",
		},
		ContainerScope: "ce",
		ContainerImage: "ce01",
		Upgrade:        false,
	},
	{
		Config: Config{
			Image: "ee01",
			Scope: AnyScope,
		},
		ContainerScope: "ce",
		ContainerImage: "ce01",
		Upgrade:        true,
	},
	{
		Config: Config{
			Image: "ee02",
			Scope: AnyScope,
		},
		ContainerScope: "ee",
		ContainerImage: "ee03",
		Upgrade:        true,
	},
	{
		Config: Config{
			Image: "ee02",
			Scope: AnyScope,
		},
		ContainerScope: "ee",
		ContainerImage: "ee02",
		Upgrade:        false,
	},
	{
		Config: Config{
			Image: "ce01",
			Scope: "ce",
		},
		ContainerScope: "ee",
		ContainerImage: "ee02",
		Upgrade:        false,
	},
	{
		Config: Config{
			Image: "ce01",
			Scope: "ce",
		},
		ContainerScope: AnyScope,
		ContainerImage: "ee02",
		Upgrade:        true,
	},
}

func TestUpgrades(t *testing.T) {
	for i, u := range upgrades {
		if u.Config.ShouldUpgrade(u.ContainerImage, u.ContainerScope) != u.Upgrade {
			t.Errorf("%d should equal %v", i, u.Upgrade)
		}
	}
}
