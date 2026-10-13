package sim

// IdlePolicy is a PlayerPolicy that never acts. A run with it has no player: only the welcome
// bonus and the daily reward move coins, so it isolates what the NPC crowd does to the Counter.
type IdlePolicy struct{}

// NewIdlePolicy returns an IdlePolicy ready to use.
func NewIdlePolicy() IdlePolicy {
	return IdlePolicy{}
}

// RunSession implements PlayerPolicy.
func (IdlePolicy) RunSession(Session) {}
