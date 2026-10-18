package updater

import (
	"testing"
)

func TestDetectEnvironment(t *testing.T) {
	// 1. Debug mode via flag
	env := DetectEnvironment("v1.0.0", "abc1234", true)
	if !env.IsDebug || env.Type != EnvDebug || env.CanSelfUpdate {
		t.Errorf("Expected EnvDebug for debugFlag=true, got %+v", env)
	}

	// 2. Debug mode via dev version
	env = DetectEnvironment("dev", "abc1234", false)
	if !env.IsDebug || env.Type != EnvDebug || env.CanSelfUpdate {
		t.Errorf("Expected EnvDebug for version=dev, got %+v", env)
	}

	// 3. Debug mode via empty/dev commit
	env = DetectEnvironment("v1.0.0", "dev", false)
	if !env.IsDebug || env.Type != EnvDebug || env.CanSelfUpdate {
		t.Errorf("Expected EnvDebug for commit=dev, got %+v", env)
	}

	// 4. Standalone binary (normal release)
	// (Assuming test binary path isn't in Cellar/Docker)
	env = DetectEnvironment("v1.0.0", "abc1234", false)
	if env.Type == EnvDebug {
		t.Errorf("Did not expect EnvDebug for v1.0.0 release build, got %+v", env)
	}
}
