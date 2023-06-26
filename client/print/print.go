package print

import "fmt"

func PanicOn(err error) {
	if err != nil {
		panic(err)
	}
}

// P is a shortcut for a call to fmt.Printf that implicitly starts
// and ends its message with a newline.
func P(format string, stuff ...interface{}) {
	fmt.Printf("\n "+format+"\n", stuff...)
}
