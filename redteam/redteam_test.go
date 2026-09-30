package redteam

import "testing"

func TestGeneratorFor(t *testing.T) {
	for _, at := range []AttackType{AttackInjection, AttackJailbreak, AttackLeakage, AttackHallucination, AttackOfftopic} {
		g, ok := GeneratorFor(at)
		if !ok || g.Type() != at {
			t.Errorf("%s: ok=%v", at, ok)
		}
	}
	if _, ok := GeneratorFor("bias"); ok {
		t.Error("bias is not an attack type this package has")
	}
}
