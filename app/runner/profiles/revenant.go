package profiles

// Revenant returns the memory profile for the Revenant server.
//
// Addresses are absolute virtual addresses (they include the process
// base), so ReadProcessMemory should use baseAddr=0.
func Revenant() Profile {
	return Profile{
		Name: "Revenant",

		CurrentHPAddr: 0x015FF908,
		MaxHPAddr:     0x015FF90C,
		CurrentSPAddr: 0x015FF910,
		MaxSPAddr:     0x015FF914,

		ZenyAddr:    0x015FBA90,
		CoordXAddr:  0x015E8184,
		CoordYAddr:  0x015E8188,
		NameAddr:    0x01602568,

		MaxWeightAddr:     0x015FBA9C,
		CurrentWeightAddr: 0x015FBAA0,
		InventorySizeAddr: 0x015FBAB4,

		MapNameAddr: 0x015FB9AC,
	}
}
