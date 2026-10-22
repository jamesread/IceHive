package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/google/go-querystring/query"
)

const (
	maxPackagesFetch    = 100
	maxPackagesInEntity = 50
)

var githubPackageTypes = []string{
	"container",
	"docker",
	"npm",
	"maven",
	"rubygems",
	"nuget",
}

// packagesSnapshot holds GitHub Packages linked to a repo or an API error string if listing failed.
type packagesSnapshot struct {
	Err         string
	Packages    []repoPackageSummary
	FetchCapped bool
}

type namespacePackagesEntry struct {
	Err      string
	Packages []ghRepoPackage
}

type repoPackageSummary struct {
	PackageType      string `json:"package_type"`
	Name             string `json:"name"`
	LatestVersion    string `json:"latest_version"`
	HTMLURL          string `json:"html_url,omitempty"`
	VersionUpdatedAt string `json:"version_updated_at,omitempty"`
}

type ghPackageOwner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type ghPackageRepository struct {
	FullName string `json:"full_name"`
}

type ghRepoPackage struct {
	Owner       *ghPackageOwner      `json:"owner"`
	Repository  *ghPackageRepository `json:"repository"`
	Name        string               `json:"name"`
	PackageType string               `json:"package_type"`
	HTMLURL     string               `json:"html_url"`
}

type ghPackageVersion struct {
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	Name      string     `json:"name"`
}

type listNamespacePackagesOpts struct {
	PackageType string `url:"package_type,omitempty"`
	Page        int    `url:"page,omitempty"`
	PerPage     int    `url:"per_page,omitempty"`
}

type listPackageVersionsOpts struct {
	Page    int `url:"page,omitempty"`
	PerPage int `url:"per_page,omitempty"`
}

func namespaceCacheKey(ownerLogin, ownerType string) string {
	return strings.ToLower(strings.TrimSpace(ownerType)) + "\x00" + strings.ToLower(strings.TrimSpace(ownerLogin))
}

func repoOwnerType(repo *github.Repository) string {
	if repo != nil {
		if o := repo.GetOwner(); o != nil && strings.EqualFold(strings.TrimSpace(o.GetType()), "Organization") {
			return "Organization"
		}
	}
	return "User"
}

func packageBelongsToRepo(pkg ghRepoPackage, repoFullName string) bool {
	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" {
		return false
	}
	if pkg.Repository == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(pkg.Repository.FullName), repoFullName)
}

func namespacePackagesPath(ownerLogin, ownerType string) string {
	ownerLogin = strings.TrimSpace(ownerLogin)
	if strings.EqualFold(strings.TrimSpace(ownerType), "Organization") {
		return fmt.Sprintf("orgs/%s/packages", ownerLogin)
	}
	return fmt.Sprintf("users/%s/packages", ownerLogin)
}

func appendURLQuery(path string, opts any) (string, error) {
	v, err := query.Values(opts)
	if err != nil {
		return "", err
	}
	if enc := v.Encode(); enc != "" {
		return path + "?" + enc, nil
	}
	return path, nil
}

func listNamespacePackages(
	ctx context.Context,
	gh *github.Client,
	ownerLogin, ownerType string,
	opts *listNamespacePackagesOpts,
) ([]ghRepoPackage, *github.Response, error) {
	u, err := appendURLQuery(namespacePackagesPath(ownerLogin, ownerType), opts)
	if err != nil {
		return nil, nil, err
	}
	req, err := gh.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	var pkgs []ghRepoPackage
	resp, err := gh.Do(ctx, req, &pkgs)
	return pkgs, resp, err
}

func listPackageVersions(
	ctx context.Context,
	gh *github.Client,
	ownerLogin, ownerType, packageType, packageName string,
	opts *listPackageVersionsOpts,
) ([]ghPackageVersion, *github.Response, error) {
	u := packageVersionsPath(ownerLogin, ownerType, packageType, packageName)
	v, err := query.Values(opts)
	if err != nil {
		return nil, nil, err
	}
	if enc := v.Encode(); enc != "" {
		u += "?" + enc
	}
	req, err := gh.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	var versions []ghPackageVersion
	resp, err := gh.Do(ctx, req, &versions)
	return versions, resp, err
}

func packageVersionsPath(ownerLogin, ownerType, packageType, packageName string) string {
	ownerLogin = strings.TrimSpace(ownerLogin)
	packageType = strings.TrimSpace(packageType)
	nameSeg := url.PathEscape(strings.TrimSpace(packageName))
	if strings.EqualFold(strings.TrimSpace(ownerType), "Organization") {
		return fmt.Sprintf("orgs/%s/packages/%s/%s/versions", ownerLogin, packageType, nameSeg)
	}
	return fmt.Sprintf("users/%s/packages/%s/%s/versions", ownerLogin, packageType, nameSeg)
}

func preferPackageVersion(name string, candidateAt, updatedAt time.Time) bool {
	return name == "" || candidateAt.After(updatedAt)
}

func latestPackageVersion(versions []ghPackageVersion) (name string, updatedAt time.Time) {
	for _, ver := range versions {
		verName := strings.TrimSpace(ver.Name)
		if verName == "" {
			continue
		}
		candidateAt := versionTimestamp(ver)
		if preferPackageVersion(name, candidateAt, updatedAt) {
			name = verName
			updatedAt = candidateAt
		}
	}
	return name, updatedAt
}

func nonZeroUTC(t *time.Time) (time.Time, bool) {
	if t == nil || t.IsZero() {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func versionTimestamp(v ghPackageVersion) time.Time {
	if at, ok := nonZeroUTC(v.UpdatedAt); ok {
		return at
	}
	at, _ := nonZeroUTC(v.CreatedAt)
	return at
}

// firstNonEmpty returns the first value that is not blank after trimming.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

func packageOwnerLoginAndType(pkg ghRepoPackage, namespaceLogin, namespaceType string) (login, ownerType string) {
	var owner ghPackageOwner
	if pkg.Owner != nil {
		owner = *pkg.Owner
	}
	return firstNonEmpty(owner.Login, namespaceLogin), firstNonEmpty(owner.Type, namespaceType, "User")
}

func isGitHubHTTPStatus(err error, code int) bool {
	var e *github.ErrorResponse
	return errors.As(err, &e) && e.Response != nil && e.Response.StatusCode == code
}

// packagesUnavailable reports errors that mean "no packages of this type here" rather than a failed run.
func packagesUnavailable(err error) bool {
	return isGitHubHTTPStatus(err, http.StatusNotFound) || isGitHubHTTPStatus(err, http.StatusForbidden)
}

func hasNextPage(resp *github.Response) bool {
	return resp != nil && resp.NextPage != 0
}

func listNamespacePackagesPage(
	ctx context.Context,
	gh *github.Client,
	ownerLogin, ownerType, pkgType, sourceID string,
	page int,
) (pkgs []ghRepoPackage, more bool, err error) {
	opts := &listNamespacePackagesOpts{PackageType: pkgType, Page: page, PerPage: 100}
	var resp *github.Response
	err = observeFetchCtx(ctx, sourceID, "packages.list_namespace", func(c context.Context) error {
		var innerErr error
		pkgs, resp, innerErr = listNamespacePackages(c, gh, ownerLogin, ownerType, opts)
		return innerErr
	})
	if packagesUnavailable(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return pkgs, hasNextPage(resp), nil
}

// loadNamespacePackagesOfType pages through one package_type until there are no more pages or limit is reached.
func loadNamespacePackagesOfType(ctx context.Context, gh *github.Client, ownerLogin, ownerType, pkgType, sourceID string, limit int) ([]ghRepoPackage, error) {
	var out []ghRepoPackage
	for page := 1; len(out) < limit; page++ {
		pkgs, more, err := listNamespacePackagesPage(ctx, gh, ownerLogin, ownerType, pkgType, sourceID, page)
		if err != nil {
			return nil, err
		}
		out = append(out, pkgs...)
		if !more {
			break
		}
	}
	return out, nil
}

func capPackages(all []ghRepoPackage) []ghRepoPackage {
	if len(all) > maxPackagesFetch {
		return all[:maxPackagesFetch]
	}
	return all
}

// loadNamespacePackages lists packages for a user/org namespace (all supported package_type values).
func loadNamespacePackages(ctx context.Context, gh *github.Client, ownerLogin, ownerType, sourceID string) ([]ghRepoPackage, error) {
	var all []ghRepoPackage
	for _, pkgType := range githubPackageTypes {
		if len(all) >= maxPackagesFetch {
			break
		}
		pkgs, err := loadNamespacePackagesOfType(ctx, gh, ownerLogin, ownerType, pkgType, sourceID, maxPackagesFetch-len(all))
		if err != nil {
			return nil, err
		}
		all = append(all, pkgs...)
	}
	return capPackages(all), nil
}

func namespaceIdentity(repo *github.Repository) (login, ownerType, errMsg string) {
	if repo == nil {
		return "", "", "nil repository"
	}
	login = repoOwnerLogin(repo)
	if login == "" {
		return "", "", "repository has no owner login"
	}
	return login, repoOwnerType(repo), ""
}

func namespacePackagesFromCache(
	ctx context.Context,
	gh *github.Client,
	repo *github.Repository,
	cache map[string]namespacePackagesEntry,
	sourceID string,
) namespacePackagesEntry {
	ownerLogin, ownerType, errMsg := namespaceIdentity(repo)
	if errMsg != "" {
		return namespacePackagesEntry{Err: errMsg}
	}
	key := namespaceCacheKey(ownerLogin, ownerType)
	if ent, ok := cache[key]; ok {
		return ent
	}
	pkgs, err := loadNamespacePackages(ctx, gh, ownerLogin, ownerType, sourceID)
	ent := namespacePackagesEntry{Packages: pkgs}
	if err != nil {
		ent.Err = err.Error()
	}
	cache[key] = ent
	return ent
}

//gocyclo:ignore
func summarizePackagesForRepo(
	ctx context.Context,
	gh *github.Client,
	repo *github.Repository,
	namespacePkgs []ghRepoPackage,
	namespaceLogin, namespaceType, sourceID string,
) packagesSnapshot {
	repoFullName := strings.TrimSpace(repo.GetFullName())
	var summaries []repoPackageSummary
	fetchCapped := false
	for _, pkg := range namespacePkgs {
		if !packageBelongsToRepo(pkg, repoFullName) {
			continue
		}
		if len(summaries) >= maxPackagesFetch {
			fetchCapped = true
			break
		}
		pkgName := strings.TrimSpace(pkg.Name)
		pkgType := strings.TrimSpace(pkg.PackageType)
		if pkgName == "" || pkgType == "" {
			continue
		}
		ownerLogin, ownerType := packageOwnerLoginAndType(pkg, namespaceLogin, namespaceType)
		var versions []ghPackageVersion
		err := observeFetchCtx(ctx, sourceID, "packages.list_versions", func(c context.Context) error {
			var innerErr error
			versions, _, innerErr = listPackageVersions(c, gh, ownerLogin, ownerType, pkgType, pkgName, &listPackageVersionsOpts{
				Page:    1,
				PerPage: 30,
			})
			return innerErr
		})
		if err != nil {
			return packagesSnapshot{Err: err.Error()}
		}
		latestName, latestAt := latestPackageVersion(versions)
		summary := repoPackageSummary{
			PackageType:   pkgType,
			Name:          pkgName,
			LatestVersion: latestName,
			HTMLURL:       strings.TrimSpace(pkg.HTMLURL),
		}
		if !latestAt.IsZero() {
			summary.VersionUpdatedAt = latestAt.Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}
	return packagesSnapshot{Packages: summaries, FetchCapped: fetchCapped}
}

func fetchPackagesForRepo(
	ctx context.Context,
	gh *github.Client,
	repo *github.Repository,
	cache map[string]namespacePackagesEntry,
	sourceID string,
) packagesSnapshot {
	ent := namespacePackagesFromCache(ctx, gh, repo, cache, sourceID)
	if ent.Err != "" {
		return packagesSnapshot{Err: ent.Err}
	}
	ownerLogin := repoOwnerLogin(repo)
	return summarizePackagesForRepo(ctx, gh, repo, ent.Packages, ownerLogin, repoOwnerType(repo), sourceID)
}

func packagesForEntity(list []repoPackageSummary, fetchCapped bool) (out []repoPackageSummary, truncated bool) {
	if len(list) <= maxPackagesInEntity {
		return list, fetchCapped
	}
	return append([]repoPackageSummary(nil), list[:maxPackagesInEntity]...), true
}

// newestPackageSummary picks the versioned package with the newest VersionUpdatedAt;
// packages without a parseable timestamp only win when no other package has one.
func newerSummary(found bool, at, bestAt time.Time) bool {
	return !found || at.After(bestAt)
}

func newestPackageSummary(list []repoPackageSummary) (repoPackageSummary, bool) {
	var best repoPackageSummary
	var bestAt time.Time
	found := false
	for _, p := range list {
		if strings.TrimSpace(p.LatestVersion) == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339, p.VersionUpdatedAt)
		if newerSummary(found, at, bestAt) {
			best, bestAt, found = p, at, true
		}
	}
	return best, found
}
