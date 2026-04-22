package version

// Name is the distribution name of the binaries.
const Name = "avf-vending-api"

// Version is injected at link time with -ldflags, otherwise "dev".
var Version = "dev"

// Commit is the git SHA injected at link time with -ldflags, otherwise empty.
var Commit = ""

// BuildTime is the build timestamp injected at link time with -ldflags, otherwise empty.
var BuildTime = ""
