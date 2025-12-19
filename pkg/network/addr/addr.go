package addr

import (
	"fmt"
	"strconv"
	"strings"
)

func SplitHostPort(hostPort string) (string, int, error) {
	hostPortStr := strings.Split(hostPort, ":")
	if len(hostPortStr) != 2 {
		return "", 0, fmt.Errorf("SplitHostPort failed,%v is not correct", hostPort)
	}
	portInt, err := strconv.ParseInt(hostPortStr[1], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return hostPortStr[0], int(portInt), nil
}
