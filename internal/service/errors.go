package service

import "errors"

var (
	ErrUnauthorized = errors.New("未登录或登录已过期")
	ErrRateLimited  = errors.New("请求过于频繁，请稍后再试")
)
