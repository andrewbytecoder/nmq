package utils

import "os"

// PathExist 判断路径是否存在
func PathExist(path string) bool {
	// 获取文件信息
	fileInfo, err := os.Stat(path)

	// 如果有错误发生，说明文件/文件夹不存在
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		// 其他错误（例如权限问题）
		return false
	}

	// 确认它是一个目录而不是文件
	if fileInfo.IsDir() {
		return true
	} else {
		return false
	}
}

func FileExists(filename string) bool {
	// os.Stat 获取文件信息
	_, err := os.Stat(filename)
	if err == nil {
		// err 为 nil，表示文件存在
		return true
	}
	// 使用 os.IsNotExist 检查错误是否为 "文件不存在"
	if os.IsNotExist(err) {
		return false
	}
	// 其他错误（如权限不足），这里你可以根据需要处理
	// 例如，返回 false 或者返回一个错误
	return false // 或者可以返回一个错误，取决于你的需求
}
