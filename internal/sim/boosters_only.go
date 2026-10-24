package sim

// BoostersOnlyPolicy is a PlayerPolicy that opens boosters and nothing else. A run with it is the
// slowest way to a binder, the baseline the Counter and the market are measured against.
type BoostersOnlyPolicy struct{}

// NewBoostersOnlyPolicy returns a BoostersOnlyPolicy ready to use.
func NewBoostersOnlyPolicy() BoostersOnlyPolicy {
	return BoostersOnlyPolicy{}
}

// RunSession implements PlayerPolicy. It opens boosters for the set with the lowest
// commons+uncommons progress, like CollectorPolicy, and never sells or buys.
func (BoostersOnlyPolicy) RunSession(s Session) {
	openBoostersWithEnergy(s)
}
