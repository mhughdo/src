package command

import "errors"

var (
	ErrSyntaxError     = errors.New("ERR syntax error")
	ErrInvalidStreamID = errors.New("ERR Invalid stream ID specified as stream command argument")
)
