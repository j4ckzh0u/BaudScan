//go:build !darwin && !linux && !windows

package ports

import "errors"

func list() ([]Port, error) {
	return nil, errors.New("此平台暂不支持自动枚举串口")
}
