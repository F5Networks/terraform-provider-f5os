package provider

import (
	"net"
	"strings"

	"golang.org/x/mod/semver"
)

// platformVersionAtLeast returns true if the device platform version is >= the
// given minimum version. Both the platform version (e.g., "1.8.3-23453") and
// the minimum (e.g., "v1.7") are normalized to semver for comparison.
// Returns false if the platform version is empty or not valid semver.
func platformVersionAtLeast(platformVersion, minimum string) bool {
	if platformVersion == "" {
		return false
	}
	if !strings.HasPrefix(platformVersion, "v") {
		platformVersion = "v" + platformVersion
	}
	return semver.Compare(semver.MajorMinor(platformVersion), minimum) >= 0
}

func extractSubnet(cidr string) (int, string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, "", err
	}

	ones, _ := ipNet.Mask.Size()
	return ones, ip.String(), nil
}

func getIntSliceDifference(slice1, slice2 []int64) []int64 {
	var diff []int64
	for _, num1 := range slice1 {
		found := false
		for _, num2 := range slice2 {
			if num1 == num2 {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, num1)
		}
	}
	return diff
}
