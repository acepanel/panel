package stalwart

import "errors"

func asDNSError[T error](err error, target *T) bool {
	return errors.As(err, target)
}
