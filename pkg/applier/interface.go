package applier

// Applier applies updates to software
type Applier interface {
	// Apply applies an update from source to target
	// source is the path to the downloaded update file
	// target is the path where the update should be applied
	Apply(source string, target string) error
}
