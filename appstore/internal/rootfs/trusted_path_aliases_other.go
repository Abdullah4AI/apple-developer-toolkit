//go:build !darwin

package rootfs

func trustedPathAliases() []string {
	return nil
}
