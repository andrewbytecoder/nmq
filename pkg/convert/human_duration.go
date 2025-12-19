package convert

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrewbytecoder/nmq/pkg/check"
)

func HumanDuration(d string) (time.Duration, error) {
	d = strings.TrimSpace(d)
	// 因为配置最常用的是没有d的配置，因此将没有d的解析提前，能够提升性能
	dr, err := time.ParseDuration(d)
	if err == nil {
		return dr, nil
	}

	if strings.Contains(d, "d") {
		index := strings.Index(d, "d")

		if check.IsNumeric(d[:index]) == false {
			return 0, fmt.Errorf("invalid day value")
		}

		day, _ := strconv.Atoi(d[:index])
		dr = time.Hour * 24 * time.Duration(day)
		ndr, err := time.ParseDuration(d[index+1:])
		if err != nil {
			return dr, nil
		}
		return dr + ndr, nil
	}

	dv, err := strconv.ParseInt(d, 10, 64)
	return time.Duration(dv), err
}
