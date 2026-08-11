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
	StepShip      StepKind = "ship"
	StepBump      StepKind = "bump"
	StepTag       StepKind = "tag"
	StepSyncCerts StepKind = "sync_certs"
	StepNotify    StepKind = "notify"
)

// ArtifactKind is the binary format for a build (android: aab default, apk optional).
type ArtifactKind string

const (
	ArtifactAAB ArtifactKind = "aab"
	ArtifactAPK ArtifactKind = "apk"
	ArtifactIPA ArtifactKind = "ipa"
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
	// ArtifactKind is aab|apk for android builds (default aab for release).
	ArtifactKind ArtifactKind
	// Flavor is a Flutter/Android product flavor (build … flavor:prod).
	Flavor string
	// Scheme is an iOS Xcode scheme; for Flutter this maps to --flavor when Flavor is empty.
	Scheme string
	// SignWith is "keystore" or "cert".
	SignWith string
	// EnvRef is the environment variable name (without env: prefix).
	EnvRef string
	// UploadTarget is play_store, testflight, or app_store.
	UploadTarget string
	Track        string
	// Rollout is a staged Play rollout fraction in (0,1], 0 means full/completed.
	Rollout float64
	// ShipFrom is "last" or an explicit artifact path.
	ShipFrom string
	// ReleaseNameStrategy: version|version_build|version_code|semver_plus|name_version|date|version_date|git_tag|git_sha|custom|none
	ReleaseNameStrategy string
	// ReleaseNameCustom is used when strategy is custom (or DSL quoted literal).
	ReleaseNameCustom string
	// NotesMode: default|none|text|file
	NotesMode string
	NotesText string
	NotesFile string
	// NotesLocale defaults to en-US for Play LocalizedText.
	NotesLocale string
	BumpLevel   BumpLevel
	TagPrefix   string
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
