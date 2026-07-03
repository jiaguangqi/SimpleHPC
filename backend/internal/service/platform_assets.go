package service

import "fmt"

func ValidatePlatformImage(kind string, size int64, width, height int, format string) error {
	if format != "png" && format != "jpeg" {
		return fmt.Errorf("仅支持 PNG 或 JPEG 图片")
	}
	switch kind {
	case "logo":
		if size > 2*1024*1024 {
			return fmt.Errorf("Logo 文件不能超过 2 MB")
		}
		if width < 16 || height < 16 || width > 2000 || height > 2000 {
			return fmt.Errorf("Logo 尺寸需在 16×16 至 2000×2000 像素之间")
		}
	case "login-image":
		if size > 8*1024*1024 {
			return fmt.Errorf("登录页图片不能超过 8 MB")
		}
		if width < 1920 || height < 1080 {
			return fmt.Errorf("登录页图片尺寸不能低于 1920×1080")
		}
		if width > 8000 || height > 8000 {
			return fmt.Errorf("登录页图片尺寸不能超过 8000×8000")
		}
	default:
		return fmt.Errorf("未知的平台图片类型")
	}
	return nil
}
