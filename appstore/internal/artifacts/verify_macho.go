package artifacts

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/subtle"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"

	"howett.net/plist"
)

const (
	maxCapturedSignatureBytes = 64 << 20
	maxCodeDirectories        = 6
	maxSpecialSlots           = 64
	maxCodeResourcesBytes     = 16 << 20
	maxCodeDirectoryPageLog2  = 24

	magicCodeDirectory = 0xfade0c02
	slotInfoPlist      = 1
	slotResourceDir    = 3
	maxSealedBlobSlot  = 0xfff
)

var (
	oidAppleCDHashes  = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 9, 1}
	oidAppleCDHashes2 = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 9, 2}
)

// signatureCapture keeps the code signature of the primary Mach-O slice, the
// one stored first, so it can be verified afterwards.
type signatureCapture struct {
	base      int64
	sliceSize int64
	superblob []byte
}

// readMachOSignatureCapture classifies every slice, keeps the primary slice's
// code signature in capture, and returns the primary slice's classification.
// A non-nil error always comes with SignatureUnreadable.
func readMachOSignatureCapture(source io.Reader, size int64, capture *signatureCapture) (string, error) {
	slices, primary, err := readMachOSignaturesCapture(source, size, capture)
	if err != nil {
		return SignatureUnreadable, err
	}
	slice := slices[primary]
	if slice.CodeSignatureError != "" {
		return SignatureUnreadable, errors.New(slice.CodeSignatureError)
	}
	return slice.CodeSignature, nil
}

func captureSuperblob(reader *forwardReader, base, sliceSize, dataSize int64) (string, *SignerIdentity, error) {
	if dataSize > maxCapturedSignatureBytes {
		return "", nil, fmt.Errorf("code signature exceeds the %d byte verification limit", maxCapturedSignatureBytes)
	}
	data, err := reader.read(dataSize)
	if err != nil {
		return "", nil, err
	}
	reader.capture.base, reader.capture.sliceSize, reader.capture.superblob = base, sliceSize, data
	return readSuperblob(&forwardReader{source: bytes.NewReader(data), size: dataSize}, dataSize)
}

type codeDirectory struct {
	slot       uint32
	raw        []byte
	hash       crypto.Hash
	hashSize   int
	pageSize   int64
	codeLimit  int64
	nSpecial   int
	nCode      int
	hashOffset int
}

func (directory *codeDirectory) cdhash() []byte {
	hasher := directory.hash.New()
	hasher.Write(directory.raw)
	return hasher.Sum(nil)
}

// specialSlot returns the hash the directory records for special slot index.
func (directory *codeDirectory) specialSlot(index int) []byte {
	start := directory.hashOffset - index*directory.hashSize
	return directory.raw[start : start+directory.hashSize]
}

func (directory *codeDirectory) codeSlot(index int) []byte {
	start := directory.hashOffset + index*directory.hashSize
	return directory.raw[start : start+directory.hashSize]
}

func codeDirectoryHash(hashType uint8) (crypto.Hash, int, error) {
	switch hashType {
	case 1:
		return crypto.SHA1, 20, nil
	case 2:
		return crypto.SHA256, 32, nil
	case 3:
		return crypto.SHA256, 20, nil
	case 4:
		return crypto.SHA384, 48, nil
	}
	return 0, 0, errUnsupportedf("code directory hash type %d is not supported", hashType)
}

func parseCodeDirectory(slot uint32, raw []byte, sliceSize int64) (*codeDirectory, error) {
	if len(raw) < 44 || binary.BigEndian.Uint32(raw) != magicCodeDirectory {
		return nil, fmt.Errorf("code directory in slot %#x is malformed", slot)
	}
	version := binary.BigEndian.Uint32(raw[8:])
	if version < 0x20000 {
		return nil, errUnsupportedf("code directory version %#x is not supported", version)
	}
	hashAlgorithm, hashSize, err := codeDirectoryHash(raw[37])
	if err != nil {
		return nil, err
	}
	if int(raw[36]) != hashSize {
		return nil, fmt.Errorf("code directory hash size %d does not match its hash type", raw[36])
	}
	directory := &codeDirectory{
		slot:       slot,
		raw:        raw,
		hash:       hashAlgorithm,
		hashSize:   hashSize,
		hashOffset: int(binary.BigEndian.Uint32(raw[16:])),
		nSpecial:   int(binary.BigEndian.Uint32(raw[24:])),
		nCode:      int(binary.BigEndian.Uint32(raw[28:])),
		codeLimit:  int64(binary.BigEndian.Uint32(raw[32:])),
	}
	if version >= 0x20100 {
		if len(raw) < 48 {
			return nil, fmt.Errorf("code directory in slot %#x is truncated", slot)
		}
		if binary.BigEndian.Uint32(raw[44:]) != 0 {
			return nil, errUnsupportedf("scatter code directories are not supported")
		}
	}
	if version >= 0x20300 {
		if len(raw) < 64 {
			return nil, fmt.Errorf("code directory in slot %#x is truncated", slot)
		}
		if limit64 := binary.BigEndian.Uint64(raw[56:]); limit64 != 0 {
			if limit64 > uint64(sliceSize) {
				return nil, fmt.Errorf("code directory covers more than the executable")
			}
			directory.codeLimit = int64(limit64)
		}
	}
	if directory.codeLimit > sliceSize {
		return nil, fmt.Errorf("code directory covers more than the executable")
	}
	if directory.nSpecial > maxSpecialSlots {
		return nil, fmt.Errorf("code directory declares %d special slots", directory.nSpecial)
	}
	if directory.hashOffset < directory.nSpecial*hashSize || int64(directory.hashOffset)+int64(directory.nCode)*int64(hashSize) > int64(len(raw)) {
		return nil, fmt.Errorf("code directory hashes are outside the blob")
	}
	pageLog2 := raw[39]
	switch {
	case pageLog2 == 0:
		directory.pageSize = directory.codeLimit
	case pageLog2 > maxCodeDirectoryPageLog2:
		return nil, errUnsupportedf("code directory page size 2^%d is not supported", pageLog2)
	default:
		directory.pageSize = int64(1) << pageLog2
	}
	expected := int64(0)
	if directory.codeLimit > 0 {
		expected = (directory.codeLimit + directory.pageSize - 1) / directory.pageSize
	}
	if int64(directory.nCode) != expected {
		return nil, fmt.Errorf("code directory has %d page hashes for %d pages", directory.nCode, expected)
	}
	return directory, nil
}

// parseSuperblobBlobs indexes the blobs of an embedded signature by slot type.
func parseSuperblobBlobs(data []byte) (map[uint32][]byte, error) {
	if len(data) < 12 || binary.BigEndian.Uint32(data) != magicEmbeddedSignature {
		return nil, fmt.Errorf("code signature superblob is malformed")
	}
	length := int64(binary.BigEndian.Uint32(data[4:]))
	count := int64(binary.BigEndian.Uint32(data[8:]))
	if length < 12 || length > int64(len(data)) || count > maxSuperblobSlots || 12+8*count > length {
		return nil, fmt.Errorf("code signature superblob is malformed")
	}
	blobs := make(map[uint32][]byte, count)
	for slot := range count {
		kind := binary.BigEndian.Uint32(data[12+slot*8:])
		offset := int64(binary.BigEndian.Uint32(data[16+slot*8:]))
		if offset < 12+8*count || offset+8 > length {
			return nil, fmt.Errorf("code signature slot is outside the superblob")
		}
		blobLength := int64(binary.BigEndian.Uint32(data[offset+4:]))
		if blobLength < 8 || blobLength > length-offset {
			return nil, fmt.Errorf("code signature blob in slot %#x has an invalid length", kind)
		}
		if _, duplicate := blobs[kind]; duplicate {
			return nil, fmt.Errorf("code signature has duplicate slot %#x", kind)
		}
		blobs[kind] = data[offset : offset+blobLength]
	}
	return blobs, nil
}

// codeSignatureInputs is what verifyCodeSignature needs besides the capture.
type codeSignatureInputs struct {
	status  string
	readErr error
	capture *signatureCapture
	// openCode reopens the executable from its first byte.
	openCode func() (io.ReadCloser, error)
	// bundle is true when the executable sits in an app bundle, so the sealed
	// Info.plist and CodeResources slots are checked against bundleFiles.
	bundle      bool
	bundleFiles map[int][]byte
}

var specialSlotNames = map[int]string{
	1: "Info.plist", 2: "requirements", 3: "CodeResources", 5: "entitlements", 7: "DER entitlements",
	8: "self launch constraint", 9: "parent launch constraint", 10: "responsible launch constraint", 11: "library constraint",
}

// embeddedBlobSlots are the special slots whose sealed data is a blob stored in
// the signature itself rather than a bundle file or external data, per the
// CSSLOT_* constants in the macOS SDK's kern/cs_blobs.h. Slots 4 (application)
// and 6 are not embedded blobs.
var embeddedBlobSlots = []int{2, 5, 7, 8, 9, 10, 11}

func specialSlotName(index int) string {
	if name, ok := specialSlotNames[index]; ok {
		return name
	}
	return fmt.Sprintf("special slot %d", index)
}

// verifyCodeSignature checks a captured Mach-O code signature: the CMS
// signature over the primary code directory, the binding of alternate code
// directories, every code directory's page and special-slot hashes, and the
// signer's chain.
func verifyCodeSignature(inputs codeSignatureInputs, policy *trustPolicy) SignatureVerification {
	switch inputs.status {
	case SignatureUnsigned:
		return SignatureVerification{Status: VerificationInvalid, Detail: "main executable has no code signature"}
	case SignatureUnreadable, "":
		detail := "code signature could not be read"
		if inputs.readErr != nil {
			detail += ": " + inputs.readErr.Error()
		}
		return SignatureVerification{Status: VerificationUnsupported, Detail: detail}
	}
	if inputs.capture == nil || inputs.capture.superblob == nil {
		return SignatureVerification{Status: VerificationUnsupported, Detail: "code signature was not captured for verification"}
	}
	blobs, err := parseSuperblobBlobs(inputs.capture.superblob)
	if err != nil {
		return failedVerification(err)
	}
	directories, err := codeDirectories(blobs, inputs.capture.sliceSize)
	if err != nil {
		return failedVerification(err)
	}
	for _, directory := range directories {
		if err := verifySpecialSlots(directory, blobs, inputs); err != nil {
			return failedVerification(err)
		}
	}
	cmsBlob := blobs[slotCMSSignature]
	var signed verifiedCMS
	if len(cmsBlob) > 8 {
		signed, err = verifyDetachedCMS(cmsBlob[8:], directories[0].raw)
		if err != nil {
			return failedVerification(err)
		}
		if err := verifyAlternateBinding(signed, directories); err != nil {
			return failedVerification(err)
		}
	}
	pages, err := verifyCodePages(inputs, directories)
	if err != nil {
		return failedVerification(err)
	}
	integrity := fmt.Sprintf("%d code director%s and %d code pages verified", len(directories), plural(len(directories), "y", "ies"), pages)
	if signed.leaf == nil {
		return SignatureVerification{Status: VerificationUntrustedChain, Detail: "ad-hoc signature has no signing certificate; " + integrity}
	}
	chain := policy.evaluateChain(signed.leaf, signed.parsed.Certificates, purposeCodeSigning)
	if chain.Status != VerificationValid {
		chain.Detail = "CMS signature and " + integrity + ", but " + chain.Detail
		return chain
	}
	chain.Detail = "CMS signature and " + integrity + "; " + chain.Detail + "; nested bundles, sealed resource files, and revocation were not checked"
	return chain
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

func codeDirectories(blobs map[uint32][]byte, sliceSize int64) ([]*codeDirectory, error) {
	primary, ok := blobs[slotCodeDirectory]
	if !ok {
		return nil, errUnsupportedf("code signature has no primary code directory")
	}
	directory, err := parseCodeDirectory(slotCodeDirectory, primary, sliceSize)
	if err != nil {
		return nil, err
	}
	directories := []*codeDirectory{directory}
	for slot := uint32(slotAlternateFirst); slot <= slotAlternateLast; slot++ {
		raw, ok := blobs[slot]
		if !ok {
			continue
		}
		if len(directories) == maxCodeDirectories {
			return nil, fmt.Errorf("code signature has more than %d code directories", maxCodeDirectories)
		}
		directory, err := parseCodeDirectory(slot, raw, sliceSize)
		if err != nil {
			return nil, err
		}
		directories = append(directories, directory)
	}
	return directories, nil
}

// verifySpecialSlots checks the hashes a code directory records for embedded
// blobs and, inside a bundle, for Info.plist and CodeResources. Every blob
// embedded in the special-slot range, and each of those bundle files that
// exists, must be sealed.
func verifySpecialSlots(directory *codeDirectory, blobs map[uint32][]byte, inputs codeSignatureInputs) error {
	for kind, blob := range blobs {
		if kind == slotCodeDirectory || kind > maxSealedBlobSlot {
			continue
		}
		if int(kind) > directory.nSpecial || isZero(directory.specialSlot(int(kind))) {
			return fmt.Errorf("embedded %s blob is not sealed by the code directory", specialSlotName(int(kind)))
		}
		if !hashMatches(directory, blob, directory.specialSlot(int(kind))) {
			return fmt.Errorf("embedded %s blob does not match its code directory hash", specialSlotName(int(kind)))
		}
	}
	// The superblob index is not signed, so a sealed blob that was stripped
	// from it must fail rather than go unchecked.
	for _, index := range embeddedBlobSlots {
		if index > directory.nSpecial || isZero(directory.specialSlot(index)) {
			continue
		}
		if _, ok := blobs[uint32(index)]; !ok {
			return fmt.Errorf("code directory seals %[1]s, but the signature has no %[1]s blob", specialSlotName(index))
		}
	}
	if !inputs.bundle {
		return nil
	}
	for _, index := range []int{slotInfoPlist, slotResourceDir} {
		data, ok := inputs.bundleFiles[index]
		if index > directory.nSpecial || isZero(directory.specialSlot(index)) {
			if ok {
				return fmt.Errorf("bundle %s is not sealed by the code directory", specialSlotName(index))
			}
			continue
		}
		if !ok {
			return fmt.Errorf("code directory seals %s, but the bundle has none", specialSlotName(index))
		}
		if !hashMatches(directory, data, directory.specialSlot(index)) {
			return fmt.Errorf("%s does not match its code directory hash", specialSlotName(index))
		}
	}
	return nil
}

func hashMatches(directory *codeDirectory, data, expected []byte) bool {
	hasher := directory.hash.New()
	hasher.Write(data)
	return subtle.ConstantTimeCompare(hasher.Sum(nil)[:directory.hashSize], expected) == 1
}

func isZero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return false
		}
	}
	return true
}

type cdHashAlgorithm struct {
	Algorithm asn1.ObjectIdentifier
	Digest    []byte
}

// verifyAlternateBinding requires each alternate code directory to be listed in
// the signed CDHashes or CDHashes2 attribute, since the CMS digest covers only
// the primary one.
func verifyAlternateBinding(signed verifiedCMS, directories []*codeDirectory) error {
	if len(directories) == 1 {
		return nil
	}
	var bound [][]byte
	var listed []byte
	if err := signed.parsed.UnmarshalSignedAttribute(oidAppleCDHashes, &listed); err == nil {
		var payload struct {
			CDHashes [][]byte `plist:"cdhashes"`
		}
		if _, err := plist.Unmarshal(listed, &payload); err == nil {
			bound = append(bound, payload.CDHashes...)
		}
	}
	for _, attribute := range signed.parsed.Signers[0].AuthenticatedAttributes {
		if !attribute.Type.Equal(oidAppleCDHashes2) {
			continue
		}
		for rest := attribute.Value.Bytes; len(rest) > 0; {
			var entry cdHashAlgorithm
			next, err := asn1.Unmarshal(rest, &entry)
			if err != nil {
				break
			}
			bound = append(bound, entry.Digest)
			rest = next
		}
	}
	for _, directory := range directories[1:] {
		cdhash := directory.cdhash()
		found := false
		for _, candidate := range bound {
			if len(candidate) >= 20 && len(candidate) <= len(cdhash) && bytes.Equal(candidate, cdhash[:len(candidate)]) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alternate code directory in slot %#x is not bound to the CMS signature", directory.slot)
		}
	}
	return nil
}

// pageChecker hashes one code directory's pages as the executable streams by.
type pageChecker struct {
	directory *codeDirectory
	hasher    hash.Hash
	position  int64
	inPage    int64
	page      int
	err       error
}

func (checker *pageChecker) Write(data []byte) (int, error) {
	written := len(data)
	directory := checker.directory
	for len(data) > 0 && checker.err == nil && checker.position < directory.codeLimit {
		take := min(int64(len(data)), directory.pageSize-checker.inPage, directory.codeLimit-checker.position)
		checker.hasher.Write(data[:take])
		data = data[take:]
		checker.inPage += take
		checker.position += take
		if checker.inPage == directory.pageSize || checker.position == directory.codeLimit {
			sum := checker.hasher.Sum(nil)[:directory.hashSize]
			if subtle.ConstantTimeCompare(sum, directory.codeSlot(checker.page)) != 1 {
				checker.err = fmt.Errorf("code page %d does not match its code directory hash", checker.page)
			}
			checker.hasher.Reset()
			checker.inPage = 0
			checker.page++
		}
	}
	return written, nil
}

func verifyCodePages(inputs codeSignatureInputs, directories []*codeDirectory) (int, error) {
	checkers := make([]io.Writer, 0, len(directories))
	limit := int64(0)
	pages := 0
	for _, directory := range directories {
		checkers = append(checkers, &pageChecker{directory: directory, hasher: directory.hash.New()})
		limit = max(limit, directory.codeLimit)
		pages = max(pages, directory.nCode)
	}
	reader, err := inputs.openCode()
	if err != nil {
		return 0, errUnsupportedf("reopen main executable: %v", err)
	}
	defer reader.Close()
	if _, err := io.CopyN(io.Discard, reader, inputs.capture.base); err != nil {
		return 0, errUnsupportedf("read main executable: %v", err)
	}
	if _, err := io.CopyN(io.MultiWriter(checkers...), reader, limit); err != nil {
		return 0, errUnsupportedf("read main executable pages: %v", err)
	}
	for _, writer := range checkers {
		if checker := writer.(*pageChecker); checker.err != nil {
			return 0, checker.err
		}
	}
	return pages, nil
}

// verifyIPACodeSignature verifies the main executable's signature, including
// the sealed Info.plist and CodeResources files of the app bundle.
func verifyIPACodeSignature(source io.ReaderAt, files []*zip.File, appRoot string, infoPlist, executable *zip.File, status string, readErr error, capture *signatureCapture, policy *trustPolicy) SignatureVerification {
	if status != SignatureSigned && status != SignatureAdHoc {
		return verifyCodeSignature(codeSignatureInputs{status: status, readErr: readErr}, policy)
	}
	codeResources, err := findMember(files, appRoot+"_CodeSignature/CodeResources")
	if err != nil {
		return SignatureVerification{Status: VerificationInvalid, Detail: err.Error()}
	}
	bundleFiles := map[int][]byte{}
	for index, member := range map[int]*zip.File{slotInfoPlist: infoPlist, slotResourceDir: codeResources} {
		if member == nil {
			continue
		}
		data, err := readZipMember(member, maxCodeResourcesBytes)
		if err != nil {
			return SignatureVerification{Status: VerificationUnsupported, Detail: fmt.Sprintf("read %s: %v", specialSlotName(index), err)}
		}
		bundleFiles[index] = data
	}
	return verifyCodeSignature(codeSignatureInputs{
		status:      status,
		capture:     capture,
		openCode:    func() (io.ReadCloser, error) { return openExecutable(source, executable) },
		bundle:      true,
		bundleFiles: bundleFiles,
	}, policy)
}

// findMember returns the one file named name. Duplicate entries are rejected
// because ZIP readers disagree on which of them is used.
func findMember(files []*zip.File, name string) (*zip.File, error) {
	var found *zip.File
	for _, file := range files {
		if zipMemberName(file.Name) == name && !file.FileInfo().IsDir() {
			if found != nil {
				return nil, fmt.Errorf("IPA has duplicate %s entries", path.Base(name))
			}
			found = file
		}
	}
	return found, nil
}

func readZipMember(file *zip.File, limit int64) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("exceeds %d bytes", limit)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("exceeds %d bytes", limit)
	}
	return data, nil
}

func openExecutable(source io.ReaderAt, file *zip.File) (io.ReadCloser, error) {
	if file.Method == zip.Store && file.CompressedSize64 == file.UncompressedSize64 {
		if offset, err := file.DataOffset(); err == nil {
			return io.NopCloser(io.NewSectionReader(source, offset, int64(file.UncompressedSize64))), nil
		}
	}
	return file.Open()
}

// verifyMachOBytes verifies a standalone Mach-O executable outside a bundle.
func verifyMachOBytes(data []byte, policy *trustPolicy) SignatureVerification {
	capture := &signatureCapture{}
	status, err := readMachOSignatureCapture(bytes.NewReader(data), int64(len(data)), capture)
	return verifyCodeSignature(codeSignatureInputs{
		status:   status,
		readErr:  err,
		capture:  capture,
		openCode: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil },
	}, policy)
}
