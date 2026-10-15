package config

// CheckTraitsForTest exposes checkTraits for white-box tests in config_test. knownNames is the
// sorted list of trait names the registry accepts; tests pass a custom list so they do not depend
// on the live DefaultTraits registry.
func CheckTraitsForTest(key string, a ArchetypeConfig, knownNames []string) error {
	return checkTraits(key, a, knownNames)
}
