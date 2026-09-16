//go:build darwin

package signing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type persistentSigningUtilityRunner func(context.Context, []byte, ...string) ([]byte, []byte, error)

func platformSigningKeychainInstallDeps() signingKeychainInstallDeps {
	runDeps := platformSigningRunDeps()
	return signingKeychainInstallDeps{
		GOOS:                      "darwin",
		SecurityAvailable:         signingRunSecurityAvailable(),
		AcquireLock:               runDeps.AcquireLock,
		CreateKeychain:            createPersistentSigningKeychain,
		ImportIdentity:            importPersistentSigningIdentity,
		KeychainSearchList:        runDeps.KeychainSearchList,
		SetKeychainSearchList:     runDeps.SetKeychainSearchList,
		RemoveKeychainSearchEntry: runDeps.RemoveKeychainSearchEntry,
		DeleteKeychain:            runDeps.DeleteKeychain,
	}
}

func createPersistentSigningKeychain(ctx context.Context, keychainPath string, password []byte) error {
	if len(password) == 0 {
		return fmt.Errorf("keychain password is empty")
	}
	if err := createPersistentKeychainWithSecurityFramework(keychainPath, password); err != nil {
		return err
	}
	return configurePersistentSigningKeychain(ctx, keychainPath, runSigningUtility, deleteSigningRunKeychain)
}

func configurePersistentSigningKeychain(
	ctx context.Context,
	keychainPath string,
	runUtility persistentSigningUtilityRunner,
	deleteKeychain func(context.Context, string) error,
) error {
	_, stderr, err := runUtility(ctx, nil, "set-keychain-settings", "-l", keychainPath)
	if err != nil {
		configureErr := utilityFailure("configure persistent keychain", stderr, err)
		if cleanupErr := deleteCreatedPersistentSigningKeychain(keychainPath, deleteKeychain); cleanupErr != nil {
			return errors.Join(configureErr, cleanupErr)
		}
		return configureErr
	}
	return nil
}

func deleteCreatedPersistentSigningKeychain(keychainPath string, deleteKeychain func(context.Context, string) error) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := deleteKeychain(cleanupCtx, keychainPath); err != nil {
		return fmt.Errorf("remove unconfigured keychain: %w", err)
	}
	return nil
}

func importPersistentSigningIdentity(ctx context.Context, keychainPath string, keychainPassword, identityData, importPassword []byte, expectedSHA1 string) error {
	if err := importPKCS12WithSecurityFramework(keychainPath, identityData, importPassword); err != nil {
		return err
	}
	if err := withPersistentSigningKeychainPasswordInput(keychainPassword, func(stdin []byte) error {
		_, stderr, err := runSigningUtility(ctx, stdin, "set-key-partition-list", "-S", "apple-tool:,apple:", "-s", "-t", "private", keychainPath)
		if err != nil {
			return utilityFailure("restrict key partition list", stderr, err, keychainPassword)
		}
		return nil
	}); err != nil {
		return err
	}
	stdout, stderr, err := runSigningUtility(ctx, nil, "find-certificate", "-a", "-Z", keychainPath)
	if err != nil {
		return utilityFailure("verify imported certificate", stderr, err)
	}
	certificates := parseSigningRunCertificateFingerprints(stdout)
	if err := validatePersistentSigningCertificateFingerprints(certificates, expectedSHA1); err != nil {
		return err
	}
	_, stderr, err = runSigningUtility(ctx, nil, "find-key", "-s", "-t", "private", keychainPath)
	if err != nil {
		return utilityFailure("verify imported private key", stderr, err)
	}
	return verifyPersistentSigningIdentityUsable(ctx, keychainPath, expectedSHA1)
}

func verifyPersistentSigningIdentityUsable(ctx context.Context, keychainPath, expectedSHA1 string) (resultErr error) {
	return withPersistentSigningProbe(
		ctx,
		keychainPath,
		expectedSHA1,
		createSigningRunTempDir,
		removeSigningRunTempDir,
		verifySigningRunIdentityUsable,
	)
}

func withPersistentSigningProbe(
	ctx context.Context,
	keychainPath, expectedSHA1 string,
	createTempDir func() (string, error),
	removeTempDir func(string) error,
	verify func(context.Context, string, string, string) error,
) (resultErr error) {
	probeDir, err := createTempDir()
	if err != nil {
		return fmt.Errorf("create private codesign probe directory: %w", err)
	}
	defer func() {
		if err := removeTempDir(probeDir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private codesign probe directory: %w", err))
		}
	}()
	return verify(ctx, probeDir, keychainPath, expectedSHA1)
}

func validatePersistentSigningCertificateFingerprints(certificates []string, expectedSHA1 string) error {
	matches := 0
	for _, fingerprint := range certificates {
		if strings.EqualFold(fingerprint, expectedSHA1) {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("verify imported certificate: expected certificate %s exactly once, found %v", expectedSHA1, certificates)
	}
	return nil
}

func withPersistentSigningKeychainPasswordInput(password []byte, operation func([]byte) error) error {
	stdin := make([]byte, len(password)+1)
	copy(stdin, password)
	stdin[len(stdin)-1] = '\n'
	defer clear(stdin)
	return operation(stdin)
}
