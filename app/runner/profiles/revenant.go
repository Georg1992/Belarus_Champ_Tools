package profiles

// Revenant returns the memory profile for the Revenant server.
//
// Addresses are module-relative offsets (from the exe base address).
// The address reader adds the process base address at runtime so
// ASLR doesn't break them.
func Revenant() Profile {
	return Profile{
		Name: "Revenant",

		CurrentHPAddr: 0x011FF908,
		MaxHPAddr:     0x011FF90C,
		CurrentSPAddr: 0x011FF910,
		MaxSPAddr:     0x011FF914,

		ZenyAddr:    0x011FBA90,
		CoordXAddr:  0x011E8184,
		CoordYAddr:  0x011E8188,
		NameAddr:    0x01202568,

		MaxWeightAddr:     0x011FBA9C,
		CurrentWeightAddr: 0x011FBAA0,
		InventorySizeAddr: 0x011FBAB4,

		MapNameAddr: 0x011FB9AC,
	}
}
