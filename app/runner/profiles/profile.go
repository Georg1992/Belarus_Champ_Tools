// Package profiles holds memory-address configurations for different
// Ragnarok Online servers. Each server has its own memory layout, so the
// addresses for HP, SP, coordinates, etc. are defined per Profile.
//
// Usage:
//
//	profiles.Default()        // returns the built-in Revenant profile
//	profiles.ByName("Revenant") // returns a profile by server name
package profiles

import "fmt"

// Profile holds module-relative memory offsets for a specific server's
// client memory layout. The address reader adds the process base address
// at runtime so ASLR doesn't break them.
//
// To find these offsets: use Cheat Engine or a memory scanner to locate
// the HP/SP addresses in the client process, then subtract the exe base
// address to get the module-relative offset.
type Profile struct {
	Name string

	// HP/SP
	CurrentHPAddr uintptr
	MaxHPAddr     uintptr
	CurrentSPAddr uintptr
	MaxSPAddr     uintptr

	// Character Info
	ZenyAddr    uintptr
	CoordXAddr  uintptr
	CoordYAddr  uintptr
	NameAddr    uintptr

	// Weight / Inventory
	MaxWeightAddr     uintptr
	CurrentWeightAddr uintptr
	InventorySizeAddr uintptr

	// Map
	MapNameAddr uintptr
}

// Default returns the built-in default profile (Revenant).
func Default() Profile {
	return Revenant()
}

// ByName returns the profile for the given server name, or an error
// if no profile matches.
var profiles = map[string]Profile{
	"Revenant": Revenant(),
}

func ByName(name string) (Profile, error) {
	p, ok := profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown server profile %q", name)
	}
	return p, nil
}

// All returns every registered profile.
func All() []Profile {
	out := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p)
	}
	return out
}
