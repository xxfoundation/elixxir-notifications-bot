////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
)

//go:generate go run gen.go
// The above generates: GITVERSION, GLIDEDEPS, and SEMVER

func printVersion() {
	fmt.Printf("Elixxir Notifications Server v%s -- %s\n\n", SEMVER, GITVERSION)
	fmt.Printf("Dependencies:\n\n%s\n", GLIDEDEPS)
}
