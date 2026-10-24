//go:build linux && cgo

package main

// #include <malloc.h>
import "C"

// trimHeap returns glibc's free heap pages to the system after a model is unloaded.
func trimHeap() { C.malloc_trim(0) }
