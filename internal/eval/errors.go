package eval

import "fmt"

func errorf(format string, args ...interface{}) error {
	return fmt.Errorf("eval: "+format, args...)
}
