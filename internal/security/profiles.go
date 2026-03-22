package security

import "github.com/grahambrooks/devc/pkg/types"

var profiles = map[string]*types.SecurityProfile{
	"strict": {
		Name: "strict",
		Network: types.NetworkConfig{
			Mode: "none",
		},
		Resources: types.ResourceConfig{
			CPUs:      "2",
			Memory:    "4g",
			PidsLimit: 128,
		},
		DropAllCaps: true,
		AddCaps:     []string{"CHOWN", "DAC_OVERRIDE", "FOWNER"},
		RunAsUser:   "1000:1000",
	},
	"moderate": {
		Name: "moderate",
		Network: types.NetworkConfig{
			Mode: "restricted",
		},
		Resources: types.ResourceConfig{
			CPUs:      "4",
			Memory:    "8g",
			PidsLimit: 256,
		},
		DropAllCaps: true,
		AddCaps:     []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "NET_BIND_SERVICE"},
		RunAsUser:   "1000:1000",
	},
	"permissive": {
		Name: "permissive",
		Network: types.NetworkConfig{
			Mode: "host",
		},
		Resources:   types.ResourceConfig{},
		DropAllCaps: false,
		RunAsUser:   "1000:1000",
	},
}

// GetProfile returns the security profile for the given name.
// Falls back to "moderate" if not found.
func GetProfile(name string) *types.SecurityProfile {
	p, ok := profiles[name]
	if !ok {
		return profiles["moderate"]
	}
	return p
}
