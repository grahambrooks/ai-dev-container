package types

import (
	"testing"
)

func TestResolvedAgents_AgentsOnly(t *testing.T) {
	d := &DevcCustomization{Agents: []string{"claude", "copilot"}}
	got := d.ResolvedAgents()
	want := []string{"claude", "copilot"}
	if !sliceEqual(got, want) {
		t.Errorf("ResolvedAgents() = %v, want %v", got, want)
	}
}

func TestResolvedAgents_AgentOnly(t *testing.T) {
	d := &DevcCustomization{Agent: "claude"}
	got := d.ResolvedAgents()
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("ResolvedAgents() = %v, want [claude]", got)
	}
}

func TestResolvedAgents_Both(t *testing.T) {
	d := &DevcCustomization{
		Agents: []string{"copilot"},
		Agent:  "claude",
	}
	got := d.ResolvedAgents()
	want := []string{"copilot", "claude"}
	if !sliceEqual(got, want) {
		t.Errorf("ResolvedAgents() = %v, want %v", got, want)
	}
}

func TestResolvedAgents_Dedup(t *testing.T) {
	d := &DevcCustomization{
		Agents: []string{"claude", "copilot"},
		Agent:  "claude",
	}
	got := d.ResolvedAgents()
	want := []string{"claude", "copilot"}
	if !sliceEqual(got, want) {
		t.Errorf("ResolvedAgents() = %v, want %v", got, want)
	}
}

func TestResolvedAgents_Empty(t *testing.T) {
	d := &DevcCustomization{}
	got := d.ResolvedAgents()
	if len(got) != 0 {
		t.Errorf("ResolvedAgents() = %v, want empty", got)
	}
}

func TestResolvedAgents_SkipsEmptyStrings(t *testing.T) {
	d := &DevcCustomization{
		Agents: []string{"", "claude", ""},
		Agent:  "",
	}
	got := d.ResolvedAgents()
	if len(got) != 1 || got[0] != "claude" {
		t.Errorf("ResolvedAgents() = %v, want [claude]", got)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
