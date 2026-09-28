package artifacts

const cpuSubtypeMask = 0x00ffffff

type cpuArchitecture struct{ cpuType, cpuSubtype uint32 }

// lipoArchitectureNames matches the names lipo -archs prints. Pairs lipo does not
// name are reported as unknown, as lipo does.
var lipoArchitectureNames = map[cpuArchitecture]string{
	{0x0100000c, 0}: "arm64", {0x0100000c, 1}: "arm64", {0x0100000c, 2}: "arm64e",
	{0x0100000c, 3}: "arm64.x1", {0x0100000c, 4}: "arm64.x2", {0x0100000c, 12}: "arm64e.x1",
	{0x0200000c, 0}: "arm64_32", {0x0200000c, 1}: "arm64_32",
	{0x01000007, 3}: "x86_64", {0x01000007, 8}: "x86_64h",
	{7, 3}:  "i386",
	{12, 5}: "armv4t", {12, 6}: "armv6", {12, 9}: "armv7", {12, 11}: "armv7s", {12, 12}: "armv7k",
	{12, 14}: "armv6m", {12, 15}: "armv7m", {12, 16}: "armv7em",
	{18, 0}: "ppc",
}

// architectureName returns the lipo name for a Mach-O CPU type and subtype.
// Capability bits in the subtype's high byte are ignored.
func architectureName(cpuType, cpuSubtype uint32) string {
	if name, ok := lipoArchitectureNames[cpuArchitecture{cpuType, cpuSubtype & cpuSubtypeMask}]; ok {
		return name
	}
	return "unknown"
}

// signersConsistent reports whether every slice has the same signature
// classification and, when signed, the same leaf certificate.
func signersConsistent(slices []ArchitectureSignature) bool {
	for _, slice := range slices[1:] {
		if slice.CodeSignature != slices[0].CodeSignature || !sameSigner(slice.Signer, slices[0].Signer) {
			return false
		}
	}
	return true
}

func sameSigner(a, b *SignerIdentity) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.SHA256Fingerprint == b.SHA256Fingerprint
}
