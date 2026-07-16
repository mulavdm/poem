//go:build !android

// The android example only builds for GOOS=android (as a c-shared library
// embedded in the POEM Android presenter APK); this stub keeps `go build
// ./...` working on desktop platforms.
package main

import "fmt"

func main() {
	fmt.Println("build with GOOS=android -buildmode=c-shared; see POEM/android_engine")
}
