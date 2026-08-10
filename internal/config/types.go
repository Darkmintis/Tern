package config

// Platform is a mobile target OS.
type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
)

// Mode is a build configuration.
type Mode string

const (
	ModeDebug   Mode = "debug"
	ModeRelease Mode = "release"
)

// StepKind is a closed-catalog step type (ADR 0002).
type StepKind string

const (
	StepBuild     StepKind = "build"
	StepSign      StepKind = "sign"
	StepUpload    StepKind = "upload"
	StepBump      StepKind = "bump"
	StepTag       StepKind = "tag"
	StepSyncCerts StepKind = "sync_certs"
	StepNotify    StepKind = "notify"
)

// BumpLevel for version bumps.
type BumpLevel string

const (
	BumpMajor BumpLevel = "major"
	BumpMinor BumpLevel = "minor"
	BumpPatch BumpLevel = "patch"
	BumpBuild BumpLevel = "build"
)

// Step is one lane instruction in the IR.
type Step struct {
	Kind     StepKind
	Platform Platform
	Mode     Mode
	// SignWith is "keystore" or "cert".
	SignWith string
	// EnvRef is the environment variable name (without env: prefix).
	EnvRef string
	// UploadTarget is play_store, testflight, or app_store.
	UploadTarget string
	Track        string
	BumpLevel    BumpLevel
	TagPrefix    string
	// SyncAction is pull or push.
	SyncAction string
	NotifyVia  string
	Raw        string // original line for logging
}

// Lane is a named sequence of steps.
type Lane struct {
	Name  string
	Steps []Step
}

// Config is the canonical Ternfile IR.
type Config struct {
	Lanes  map[string]Lane
	Source string // path loaded from
}

// LaneNames returns lane names in stable order (insertion is not guaranteed; sorted by caller if needed).
func (c *Config) Lane(name string) (Lane, bool) {
	if c == nil || c.Lanes == nil {
		return Lane{}, false
	}
	l, ok := c.Lanes[name]
	return l, ok
}
