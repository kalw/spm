// Package versions maintains the semver-sorted versions.json manifest.
package versions

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// Parse decodes a versions.json body (a JSON array of version strings).
// An empty or nil body yields an empty list (first publish).
func Parse(body []byte) ([]string, error) {
	if len(body) == 0 {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parsing versions.json: %w", err)
	}
	return list, nil
}

// Merge adds version to list, returning the deduped, semver-sorted result.
// If the version already exists and force is false, it returns an error so the
// caller can preserve artifact immutability.
func Merge(list []string, version string, force bool) ([]string, error) {
	if _, err := semver.StrictNewVersion(version); err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", version, err)
	}
	set := map[string]bool{}
	for _, v := range list {
		set[v] = true
	}
	if set[version] && !force {
		return nil, fmt.Errorf("version %s already published (use --force to overwrite)", version)
	}
	set[version] = true
	return Sort(keys(set))
}

// Sort returns the versions ordered ascending by semver. Non-semver entries
// cause an error rather than being silently reordered.
func Sort(list []string) ([]string, error) {
	vs := make([]*semver.Version, 0, len(list))
	for _, s := range list {
		v, err := semver.StrictNewVersion(s)
		if err != nil {
			return nil, fmt.Errorf("versions.json contains non-semver entry %q: %w", s, err)
		}
		vs = append(vs, v)
	}
	sort.Sort(semver.Collection(vs))
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Original()
	}
	return out, nil
}

// Encode renders a version list as pretty JSON with a trailing newline.
func Encode(list []string) ([]byte, error) {
	if list == nil {
		list = []string{}
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
