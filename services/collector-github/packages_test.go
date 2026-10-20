package main

import (
	"testing"
	"time"
)

func TestLatestPackageVersion(t *testing.T) {
	t.Parallel()
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	name, at := latestPackageVersion([]ghPackageVersion{
		{Name: "v1.0.0", UpdatedAt: &older},
		{Name: "v2.0.0", UpdatedAt: &newer},
		{Name: "", UpdatedAt: &newer},
	})
	if name != "v2.0.0" || !at.Equal(newer) {
		t.Fatalf("got %q %v", name, at)
	}
}

func TestPackageVersionsPath(t *testing.T) {
	t.Parallel()
	got := packageVersionsPath("myorg", "Organization", "container", "app")
	want := "orgs/myorg/packages/container/app/versions"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = packageVersionsPath("user", "User", "npm", "@scope/pkg")
	if got != "users/user/packages/npm/@scope%2Fpkg/versions" {
		t.Fatalf("unexpected path: %q", got)
	}
}

func TestPackageBelongsToRepo(t *testing.T) {
	t.Parallel()
	pkg := ghRepoPackage{Repository: &ghPackageRepository{FullName: "jamesread/orgviz"}}
	if !packageBelongsToRepo(pkg, "jamesread/orgviz") {
		t.Fatal("expected match")
	}
	if packageBelongsToRepo(pkg, "other/repo") {
		t.Fatal("expected no match")
	}
	if packageBelongsToRepo(ghRepoPackage{}, "jamesread/orgviz") {
		t.Fatal("expected no repo association")
	}
}

func TestNewestPackageSummary(t *testing.T) {
	t.Parallel()
	best, ok := newestPackageSummary([]repoPackageSummary{
		{PackageType: "npm", Name: "a", LatestVersion: "1.0.0", VersionUpdatedAt: "2024-01-01T00:00:00Z"},
		{PackageType: "container", Name: "b", LatestVersion: "2.0.0", VersionUpdatedAt: "2025-01-01T00:00:00Z"},
	})
	if !ok || best.PackageType != "container" || best.LatestVersion != "2.0.0" {
		t.Fatalf("got %+v ok=%v", best, ok)
	}
}

func TestNewestPackageSummaryFallbacks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		list     []repoPackageSummary
		wantName string
		wantOK   bool
	}{
		{
			name: "dated package beats undated and unparseable",
			list: []repoPackageSummary{
				{Name: "unversioned"},
				{Name: "no-time", LatestVersion: "0.1.0"},
				{Name: "dated", LatestVersion: "1.0.0", VersionUpdatedAt: "2024-01-01T00:00:00Z"},
				{Name: "bad-time", LatestVersion: "2.0.0", VersionUpdatedAt: "yesterday"},
			},
			wantName: "dated",
			wantOK:   true,
		},
		{
			name: "first versioned package without timestamps",
			list: []repoPackageSummary{
				{Name: "unversioned"},
				{Name: "first", LatestVersion: "0.1.0"},
				{Name: "second", LatestVersion: "0.2.0"},
			},
			wantName: "first",
			wantOK:   true,
		},
		{
			name: "no versions",
			list: []repoPackageSummary{{Name: "unversioned"}},
		},
	}
	for _, tc := range cases {
		best, ok := newestPackageSummary(tc.list)
		if ok != tc.wantOK || best.Name != tc.wantName {
			t.Errorf("%s: got %+v ok=%v", tc.name, best, ok)
		}
	}
}

func TestPackageOwnerLoginAndType(t *testing.T) {
	t.Parallel()
	login, ownerType := packageOwnerLoginAndType(ghRepoPackage{Owner: &ghPackageOwner{Login: " acme ", Type: "Organization"}}, "ns", "User")
	if login != "acme" || ownerType != "Organization" {
		t.Fatalf("owner fields: got %q %q", login, ownerType)
	}
	login, ownerType = packageOwnerLoginAndType(ghRepoPackage{}, " ns ", "")
	if login != "ns" || ownerType != "User" {
		t.Fatalf("fallbacks: got %q %q", login, ownerType)
	}
}
