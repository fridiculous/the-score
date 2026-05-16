//go:build windows

package runtime

func LookupProcessCWD(pid int) (string, bool) {
	return "", false
}
