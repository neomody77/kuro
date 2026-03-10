package skill

import (
	"runtime"
	"testing"
)

func TestCheckRequirementsNil(t *testing.T) {
	if err := CheckRequirements(nil); err != nil {
		t.Errorf("nil require should pass, got: %v", err)
	}
}

func TestCheckRequirementsEnvPass(t *testing.T) {
	// PATH is always set
	err := CheckRequirements(&SkillRequire{Env: []string{"PATH"}})
	if err != nil {
		t.Errorf("expected pass for PATH env, got: %v", err)
	}
}

func TestCheckRequirementsEnvFail(t *testing.T) {
	err := CheckRequirements(&SkillRequire{Env: []string{"KURO_TEST_NONEXISTENT_VAR_12345"}})
	if err == nil {
		t.Error("expected error for missing env var")
	}
}

func TestCheckRequirementsBinsPass(t *testing.T) {
	// "sh" should always exist
	err := CheckRequirements(&SkillRequire{Bins: []string{"sh"}})
	if err != nil {
		t.Errorf("expected pass for sh binary, got: %v", err)
	}
}

func TestCheckRequirementsBinsFail(t *testing.T) {
	err := CheckRequirements(&SkillRequire{Bins: []string{"kuro_nonexistent_binary_12345"}})
	if err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestCheckRequirementsOSPass(t *testing.T) {
	err := CheckRequirements(&SkillRequire{OS: []string{runtime.GOOS}})
	if err != nil {
		t.Errorf("expected pass for current OS %q, got: %v", runtime.GOOS, err)
	}
}

func TestCheckRequirementsOSFail(t *testing.T) {
	err := CheckRequirements(&SkillRequire{OS: []string{"plan9"}})
	if err == nil {
		t.Error("expected error for wrong OS")
	}
}

func TestCheckRequirementsCombinedErrors(t *testing.T) {
	err := CheckRequirements(&SkillRequire{
		Env:  []string{"KURO_TEST_NONEXISTENT_1"},
		Bins: []string{"kuro_nonexistent_bin_1"},
	})
	if err == nil {
		t.Fatal("expected combined error")
	}
	s := err.Error()
	if !containsStr(s, "env") || !containsStr(s, "binary") {
		t.Errorf("expected combined error mentioning env and binary, got: %s", s)
	}
}
