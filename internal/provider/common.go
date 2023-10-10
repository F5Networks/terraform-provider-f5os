package provider

import "net"

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
