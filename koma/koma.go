package koma

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -lm
#include "common-koma.h"
*/
import "C"

// wrapper functions in Go
func KomaPull(fd int) int {
	return int(C.koma_pull(C.int(fd)))
}

func KomaInit() int {
	return int(C.koma_init())
}

func KomaAttach(fd int, csock int, initialConnWindow int) int {
	return int(C.koma_attach(C.int(fd), C.int(csock), C.int(initialConnWindow)))
}
