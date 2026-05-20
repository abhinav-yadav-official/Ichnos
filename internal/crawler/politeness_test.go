package crawler

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPolitenessAllowedReturnsFalseForDisallowedPath(t *testing.T) {
	politeness := NewPoliteness(10*time.Millisecond, "IchnosBot", staticRobots(map[string]string{
		"example.com": "User-agent: *\nDisallow: /private\n",
	}))

	allowed, err := politeness.Allowed(context.Background(), "https://example.com/private/page")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Fatal("disallowed path was allowed")
	}
}

func TestPolitenessAllowedReturnsTrueForAllowedPath(t *testing.T) {
	politeness := NewPoliteness(10*time.Millisecond, "IchnosBot", staticRobots(map[string]string{
		"example.com": "User-agent: *\nDisallow: /private\n",
	}))

	allowed, err := politeness.Allowed(context.Background(), "https://example.com/public/page")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if !allowed {
		t.Fatal("allowed path was disallowed")
	}
}

func TestPolitenessWaitRateLimitsPerHost(t *testing.T) {
	politeness := NewPoliteness(40*time.Millisecond, "IchnosBot", staticRobots(nil))

	ctx := context.Background()
	start := time.Now()
	if err := politeness.Wait(ctx, "example.com"); err != nil {
		t.Fatalf("first Wait returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("first Wait took %s, want immediate", elapsed)
	}

	start = time.Now()
	if err := politeness.Wait(ctx, "example.com"); err != nil {
		t.Fatalf("second Wait returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Fatalf("second Wait took %s, want rate-limited delay", elapsed)
	}
}

func TestLiveGitHubRobotsDisallowsAccountLogin(t *testing.T) {
	if os.Getenv("ICHNOS_LIVE_TESTS") != "1" {
		t.Skip("set ICHNOS_LIVE_TESTS=1 to fetch live robots.txt")
	}
	if testing.Short() {
		t.Skip("skipping live robots.txt fetch in short mode")
	}

	politeness := NewPoliteness(time.Millisecond, "IchnosBot", nil)
	allowed, err := politeness.Allowed(context.Background(), "https://github.com/account-login")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Fatal("github.com/account-login was allowed, want disallowed")
	}
}

func staticRobots(files map[string]string) robotsFetcher {
	return func(ctx context.Context, scheme, host string) (string, error) {
		return files[host], nil
	}
}
