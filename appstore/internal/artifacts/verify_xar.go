package artifacts

import (
	"bytes"
	"crypto"
	"crypto/md5" //nolint:gosec // xar records MD5 member checksums.
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // xar records SHA-1 member checksums.
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"
)

const maxXarSignatureBytes = 16 << 10

// xarChecksumHash maps the header checksum algorithm to a hash. Apple's xar
// defines 3 as SHA-256 and 4 as SHA-512; the original xar format instead uses
// 3 for a hash named in the bytes after the fixed header, which is honored
// when such a name is present.
func xarChecksumHash(algorithm uint32, name string) (crypto.Hash, string, error) {
	switch algorithm {
	case 0:
		return 0, "", fmt.Errorf("package table of contents has no checksum")
	case 1:
		return crypto.SHA1, "sha1", nil
	case 2:
		return 0, "", errUnsupportedf("package table of contents checksum md5 is not supported")
	case 3:
		switch name {
		case "", "sha256":
			return crypto.SHA256, "sha256", nil
		case "sha384":
			return crypto.SHA384, name, nil
		case "sha512":
			return crypto.SHA512, name, nil
		}
		return 0, "", errUnsupportedf("package table of contents checksum %q is not supported", name)
	case 4:
		return crypto.SHA512, "sha512", nil
	}
	return 0, "", errUnsupportedf("package table of contents checksum algorithm %d is not supported", algorithm)
}

// verifyXarSignature checks the classic RSA signature of a flat package: the
// heap checksum must match the compressed table of contents, the signature
// must cover that checksum, every member's heap bytes must match the archived
// checksum the signed table of contents records, and the signer must chain to
// an Apple root.
func verifyXarSignature(source io.ReaderAt, size int64, document *xarDocument, status string, signatureErr error, policy *trustPolicy) SignatureVerification {
	switch {
	case status == SignatureUnreadable:
		detail := "package signature could not be read"
		if signatureErr != nil {
			detail += ": " + signatureErr.Error()
		}
		return SignatureVerification{Status: VerificationUnsupported, Detail: detail}
	case document.Signature == nil && document.XSignature != nil:
		return SignatureVerification{Status: VerificationUnsupported, Detail: "package has only a CMS x-signature, which is not verified"}
	case document.Signature == nil:
		return SignatureVerification{Status: VerificationInvalid, Detail: "package is unsigned"}
	}
	checksum, err := verifiedXarChecksum(source, size, document)
	if err != nil {
		return failedVerification(err)
	}
	leaf, carried, err := verifyXarRSASignature(source, size, document, checksum)
	if err != nil {
		return failedVerification(err)
	}
	checked, err := verifyXarFileChecksums(source, size, checksum.heap, document.Files)
	if err != nil {
		return failedVerification(err)
	}
	verified := fmt.Sprintf("table of contents checksum and RSA signature verified and %d file payload %s recomputed", checked, plural(checked, "checksum", "checksums"))
	chain := policy.evaluateChain(leaf, carried, purposeInstaller)
	if chain.Status != VerificationValid {
		chain.Detail = verified + ", but " + chain.Detail
		return chain
	}
	chain.Detail = verified + "; " + chain.Detail
	if document.XSignature != nil {
		chain.Detail += "; the CMS x-signature and revocation were not checked"
	} else {
		chain.Detail += "; revocation was not checked"
	}
	return chain
}

// xarMemberHash returns the hash for a member checksum style. xar writes
// lowercase names; the comparison ignores case.
func xarMemberHash(style string) (func() hash.Hash, bool) {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "sha1":
		return sha1.New, true
	case "sha256":
		return sha256.New, true
	case "sha384":
		return sha512.New384, true
	case "sha512":
		return sha512.New, true
	case "md5":
		return md5.New, true
	}
	return nil, false
}

// xarMemberCheck is one member whose heap bytes the table of contents binds.
type xarMemberCheck struct {
	label          string
	offset, length int64
	style          string
	newHash        func() hash.Hash
	want           []byte
}

type xarHeapSpan struct {
	offset, length int64
	style          string
}

// verifyXarFileChecksums recomputes the archived checksum of every member the
// signed table of contents lists with heap data, including the members of
// component packages in a product archive, and returns how many it checked.
// Extended attributes are not checked, matching pkgutil --check-signature.
// Every range is validated before any byte is hashed: it must lie inside the
// heap, and ranges may only overlap by being identical, as deduplicated xar
// members are, so the bytes hashed never exceed the heap. Each member is
// hashed in a stream over exactly its declared range. A mismatch is reported
// before an unsupported checksum style.
func verifyXarFileChecksums(source io.ReaderAt, size, heap int64, files []xarFile) (int, error) {
	heapSize := size - heap
	if heapSize < 0 {
		return 0, fmt.Errorf("xar heap is outside the package")
	}
	var checks []xarMemberCheck
	var unsupported error
	add := func(label string, offset, length int64, archived *xarFileChecksum) error {
		check, err := xarMemberChecksum(label, offset, length, archived, heapSize)
		var unsupportedErr unsupportedError
		switch {
		case errors.As(err, &unsupportedErr):
			if unsupported == nil {
				unsupported = err
			}
		case err != nil:
			return err
		}
		checks = append(checks, check)
		return nil
	}
	var collect func(prefix string, files []xarFile) error
	collect = func(prefix string, files []xarFile) error {
		for _, file := range files {
			name := prefix + file.Name
			if file.Data.XMLName.Local != "" {
				if err := add(fmt.Sprintf("file %q", name), file.Data.Offset, file.Data.Length, file.Data.ArchivedChecksum); err != nil {
					return err
				}
			}
			if err := collect(name+"/", file.Files); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect("", files); err != nil {
		return 0, err
	}
	if err := checkXarRangesDisjoint(checks); err != nil {
		return 0, err
	}
	digests := map[xarHeapSpan][]byte{}
	checked := 0
	for _, check := range checks {
		if check.newHash == nil {
			continue
		}
		span := xarHeapSpan{offset: check.offset, length: check.length, style: check.style}
		got, seen := digests[span]
		if !seen {
			hasher := check.newHash()
			if _, err := io.Copy(hasher, io.NewSectionReader(source, heap+check.offset, check.length)); err != nil {
				return 0, fmt.Errorf("read xar %s: %w", check.label, err)
			}
			got = hasher.Sum(nil)
			digests[span] = got
		}
		if subtle.ConstantTimeCompare(got, check.want) != 1 {
			return 0, fmt.Errorf("xar %s does not match its archived %s checksum", check.label, check.style)
		}
		checked++
	}
	if unsupported != nil {
		return 0, unsupported
	}
	return checked, nil
}

// xarMemberChecksum validates a heap range and its archived checksum. label
// names the member in errors. For an unsupported style it returns the range
// with a nil hash and an error.
func xarMemberChecksum(label string, offset, length int64, archived *xarFileChecksum, heapSize int64) (xarMemberCheck, error) {
	if offset < 0 || length < 0 || offset > heapSize || length > heapSize-offset {
		return xarMemberCheck{}, fmt.Errorf("xar %s data is outside the package", label)
	}
	check := xarMemberCheck{label: label, offset: offset, length: length}
	if archived == nil {
		return xarMemberCheck{}, fmt.Errorf("xar %s has no archived checksum", label)
	}
	check.style = strings.ToLower(strings.TrimSpace(archived.Style))
	newHash, ok := xarMemberHash(check.style)
	if !ok {
		return check, errUnsupportedf("xar %s archived checksum style %q is not supported", label, archived.Style)
	}
	want, err := hex.DecodeString(strings.TrimSpace(archived.Value))
	if err != nil || len(want) != newHash().Size() {
		return xarMemberCheck{}, fmt.Errorf("xar %s archived checksum is malformed", label)
	}
	check.newHash, check.want = newHash, want
	return check, nil
}

// checkXarRangesDisjoint rejects member ranges that overlap without being
// identical.
func checkXarRangesDisjoint(checks []xarMemberCheck) error {
	ranges := make([]xarMemberCheck, 0, len(checks))
	for _, check := range checks {
		if check.length > 0 {
			ranges = append(ranges, check)
		}
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].offset != ranges[j].offset {
			return ranges[i].offset < ranges[j].offset
		}
		return ranges[i].length < ranges[j].length
	})
	end := int64(0)
	for index, current := range ranges {
		if index > 0 {
			previous := ranges[index-1]
			if current.offset == previous.offset && current.length == previous.length {
				continue
			}
			if current.offset < end {
				return fmt.Errorf("xar %s and %s have overlapping data ranges", previous.label, current.label)
			}
		}
		end = current.offset + current.length
	}
	return nil
}

type xarChecksum struct {
	hash  crypto.Hash
	value []byte
	heap  int64
}

func verifiedXarChecksum(source io.ReaderAt, size int64, document *xarDocument) (xarChecksum, error) {
	header := make([]byte, 28)
	if _, err := source.ReadAt(header, 0); err != nil {
		return xarChecksum{}, fmt.Errorf("read xar header: %w", err)
	}
	headerSize := int64(binary.BigEndian.Uint16(header[4:6]))
	tocCompressed := int64(binary.BigEndian.Uint64(header[8:16]))
	if headerSize < 28 || headerSize > size || tocCompressed < 0 || tocCompressed > maxXarTOCBytes || tocCompressed > size-headerSize {
		return xarChecksum{}, fmt.Errorf("xar header is malformed")
	}
	name := ""
	if headerSize > 28 {
		extra := make([]byte, min(headerSize-28, 64))
		if _, err := source.ReadAt(extra, 28); err != nil {
			return xarChecksum{}, fmt.Errorf("read xar header: %w", err)
		}
		if end := bytes.IndexByte(extra, 0); end >= 0 {
			extra = extra[:end]
		}
		name = strings.ToLower(string(extra))
	}
	hashAlgorithm, style, err := xarChecksumHash(binary.BigEndian.Uint32(header[24:28]), name)
	if err != nil {
		return xarChecksum{}, err
	}
	declared := document.Checksum
	if declared == nil || !strings.EqualFold(declared.Style, style) {
		return xarChecksum{}, fmt.Errorf("table of contents checksum does not match the header algorithm %s", style)
	}
	heap := headerSize + tocCompressed
	stored, err := readXarHeap(source, size, heap, *declared, int64(hashAlgorithm.Size()), "table of contents checksum")
	if err != nil {
		return xarChecksum{}, err
	}
	hasher := hashAlgorithm.New()
	if _, err := io.Copy(hasher, io.NewSectionReader(source, headerSize, tocCompressed)); err != nil {
		return xarChecksum{}, fmt.Errorf("read xar table of contents: %w", err)
	}
	if subtle.ConstantTimeCompare(hasher.Sum(nil), stored) != 1 {
		return xarChecksum{}, fmt.Errorf("table of contents checksum does not match the table of contents")
	}
	return xarChecksum{hash: hashAlgorithm, value: stored, heap: heap}, nil
}

func readXarHeap(source io.ReaderAt, size, heap int64, location xarHeapRange, wantSize int64, name string) ([]byte, error) {
	if location.Offset < 0 || location.Size <= 0 || location.Size > maxXarSignatureBytes || (wantSize > 0 && location.Size != wantSize) {
		return nil, fmt.Errorf("%s has an invalid size", name)
	}
	if location.Offset > size-heap || location.Size > size-heap-location.Offset {
		return nil, fmt.Errorf("%s is outside the package", name)
	}
	data := make([]byte, location.Size)
	if _, err := source.ReadAt(data, heap+location.Offset); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

func verifyXarRSASignature(source io.ReaderAt, size int64, document *xarDocument, checksum xarChecksum) (*x509.Certificate, []*x509.Certificate, error) {
	signature := document.Signature
	if !strings.EqualFold(signature.Style, "RSA") {
		return nil, nil, errUnsupportedf("package signature style %q is not supported", signature.Style)
	}
	certificates, err := xarCertificates(signature)
	if err != nil {
		return nil, nil, err
	}
	public, ok := certificates[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, errUnsupportedf("package signing certificate does not hold an RSA key")
	}
	value, err := readXarHeap(source, size, checksum.heap, signature.xarHeapRange, 0, "package signature")
	if err != nil {
		return nil, nil, err
	}
	// xar signs the checksum as a precomputed digest of its algorithm. Older
	// productsign releases instead signed the digest of the checksum, which
	// pkgutil also accepts; both forms bind the table of contents.
	if err := rsa.VerifyPKCS1v15(public, checksum.hash, checksum.value, value); err != nil {
		hasher := checksum.hash.New()
		hasher.Write(checksum.value)
		if err := rsa.VerifyPKCS1v15(public, checksum.hash, hasher.Sum(nil), value); err != nil {
			return nil, nil, fmt.Errorf("package RSA signature does not verify over the table of contents checksum")
		}
	}
	return certificates[0], certificates[1:], nil
}

func xarCertificates(signature *xarSignature) ([]*x509.Certificate, error) {
	if len(signature.Certificates) == 0 || len(signature.Certificates) > maxXarSignerCertCount {
		return nil, fmt.Errorf("package signature lists %d certificates", len(signature.Certificates))
	}
	certificates := make([]*x509.Certificate, 0, len(signature.Certificates))
	for _, encoded := range signature.Certificates {
		der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
		if err != nil {
			return nil, fmt.Errorf("decode package certificate: %w", err)
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse package certificate: %w", err)
		}
		certificates = append(certificates, certificate)
	}
	return certificates, nil
}
