package svncfg

import "errors"

var (
	ErrNotFound   = errors.New("目标不存在")
	ErrConflict   = errors.New("配置已发生变化")
	ErrAmbiguous  = errors.New("配置目标存在歧义")
	ErrValidation = errors.New("参数不合法")
)
