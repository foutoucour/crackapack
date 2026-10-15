package domain

// ValidColors returns the Magic colours an archetype may list, as MTGJSON writes them.
func ValidColors() []string { return []string{"W", "U", "B", "R", "G"} }

// validateColors rejects a colour outside ValidColors and a colour listed twice.
//
// Contract stub: the colour-affinity story owns this body.
func validateColors(_ []string) error { return nil }
