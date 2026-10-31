package storage

import (
	"fmt"
	"os"
	"reflect"
)

// Identity is an inode identity within its filesystem, not a content identity.
// Ambiguous hard links are resolved by path by the scanner.
func Identity(fi os.FileInfo) string {
	v := reflect.ValueOf(fi.Sys())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}
	dev, ino := v.FieldByName("Dev"), v.FieldByName("Ino")
	if !dev.IsValid() || !ino.IsValid() {
		return ""
	}
	return fmt.Sprint(dev.Interface()) + ":" + fmt.Sprint(ino.Interface())
}
