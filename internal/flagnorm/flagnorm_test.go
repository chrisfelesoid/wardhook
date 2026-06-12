package flagnorm_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/chrisfelesoid/wardhook/internal/flagnorm"
)

func sortedKeys(m map[string]struct{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func TestNormalize_BundledShortFlags(t *testing.T) {
	t.Parallel()
	flags, _, args, ok := flagnorm.Normalize([]string{"rm", "-rf", "./foo"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	got := sortedKeys(flags)
	want := []string{"f", "r"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("flags: got %v, want %v", got, want)
	}
	if !reflect.DeepEqual(args, []string{"./foo"}) {
		t.Errorf("args: got %v, want [./foo]", args)
	}
}

func TestNormalize_SeparateShortFlags(t *testing.T) {
	t.Parallel()
	flags, _, args, ok := flagnorm.Normalize([]string{"rm", "-r", "-f", "./foo"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	got := sortedKeys(flags)
	if !reflect.DeepEqual(got, []string{"f", "r"}) {
		t.Errorf("flags: got %v", got)
	}
	if !reflect.DeepEqual(args, []string{"./foo"}) {
		t.Errorf("args: got %v", args)
	}
}

func TestNormalize_LongFlagsWithAliases(t *testing.T) {
	t.Parallel()
	aliases := flagnorm.Aliases{
		"r": {"recursive"},
		"f": {"force"},
	}
	flags, _, args, ok := flagnorm.Normalize(
		[]string{"rm", "--recursive", "--force", "./foo"}, aliases, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	got := sortedKeys(flags)
	if !reflect.DeepEqual(got, []string{"f", "r"}) {
		t.Errorf("flags: got %v", got)
	}
	if !reflect.DeepEqual(args, []string{"./foo"}) {
		t.Errorf("args: got %v", args)
	}
}

func TestNormalize_KeyValueLongFlag(t *testing.T) {
	t.Parallel()
	flags, _, args, ok := flagnorm.Normalize([]string{"git", "--depth=1", "repo"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	got := sortedKeys(flags)
	if !reflect.DeepEqual(got, []string{"depth"}) {
		t.Errorf("flags: got %v, want [depth]", got)
	}
	if !reflect.DeepEqual(args, []string{"repo"}) {
		t.Errorf("args: got %v, want [repo]", args)
	}
}

func TestNormalize_DoubleDashTerminator(t *testing.T) {
	t.Parallel()
	flags, _, args, ok := flagnorm.Normalize([]string{"rm", "--", "-rf"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	if len(flags) != 0 {
		t.Errorf("flags should be empty: got %v", sortedKeys(flags))
	}
	if !reflect.DeepEqual(args, []string{"-rf"}) {
		t.Errorf("args: got %v, want [-rf]", args)
	}
}

func TestNormalize_DropsCommandWord(t *testing.T) {
	t.Parallel()
	flags, _, args, ok := flagnorm.Normalize([]string{"echo", "hello"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	if len(flags) != 0 {
		t.Errorf("flags: %v", sortedKeys(flags))
	}
	if !reflect.DeepEqual(args, []string{"hello"}) {
		t.Errorf("args: %v", args)
	}
}

func TestNormalize_EqualsFormCapturesValue_LongFlag(t *testing.T) {
	t.Parallel()
	flags, values, args, ok := flagnorm.Normalize(
		[]string{"git", "--depth=1", "repo"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	if !reflect.DeepEqual(sortedKeys(flags), []string{"depth"}) {
		t.Errorf("flags: %v", sortedKeys(flags))
	}
	if got := values["depth"]; !reflect.DeepEqual(got, []string{"1"}) {
		t.Errorf("values[depth]: %v", got)
	}
	if !reflect.DeepEqual(args, []string{"repo"}) {
		t.Errorf("args: %v", args)
	}
}

func TestNormalize_EqualsFormCapturesValue_SingleDashLong(t *testing.T) {
	t.Parallel()
	flags, values, _, ok := flagnorm.Normalize(
		[]string{"terraform", "-chdir=environments/prod", "apply"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	if !reflect.DeepEqual(sortedKeys(flags), []string{"chdir"}) {
		t.Errorf("flags: %v (expected single-dash multi-char with = to be long flag)",
			sortedKeys(flags))
	}
	if got := values["chdir"]; !reflect.DeepEqual(got, []string{"environments/prod"}) {
		t.Errorf("values[chdir]: %v", got)
	}
}

func TestNormalize_EqualsFormCapturesValue_ShortFlag(t *testing.T) {
	t.Parallel()
	_, values, _, ok := flagnorm.Normalize(
		[]string{"cmd", "-n=prod"}, nil, nil)
	if !ok {
		t.Fatal("ok=false")
	}
	if got := values["n"]; !reflect.DeepEqual(got, []string{"prod"}) {
		t.Errorf("values[n]: %v", got)
	}
}

func TestNormalize_SpaceFormCapturesValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		words       []string
		valueTaking map[string]struct{}
		wantFlags   []string
		wantValues  map[string][]string
		wantArgs    []string
		wantOK      bool
	}{
		{
			name:        "terraform -chdir env/prod apply (single-dash long)",
			words:       []string{"terraform", "-chdir", "env/prod", "apply"},
			valueTaking: map[string]struct{}{"chdir": {}},
			wantFlags:   []string{"chdir"},
			wantValues:  map[string][]string{"chdir": {"env/prod"}},
			wantArgs:    []string{"apply"},
			wantOK:      true,
		},
		{
			name:        "kubectl --namespace prod get (double-dash long)",
			words:       []string{"kubectl", "--namespace", "prod", "get"},
			valueTaking: map[string]struct{}{"namespace": {}},
			wantFlags:   []string{"namespace"},
			wantValues:  map[string][]string{"namespace": {"prod"}},
			wantArgs:    []string{"get"},
			wantOK:      true,
		},
		{
			name:        "kubectl -n prod get (single-char short)",
			words:       []string{"kubectl", "-n", "prod", "get"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n"},
			wantValues:  map[string][]string{"n": {"prod"}},
			wantArgs:    []string{"get"},
			wantOK:      true,
		},
		{
			name:        "cmd -var foo -var bar (multi-occurrence)",
			words:       []string{"cmd", "-var", "foo", "-var", "bar"},
			valueTaking: map[string]struct{}{"var": {}},
			wantFlags:   []string{"var"},
			wantValues:  map[string][]string{"var": {"foo", "bar"}},
			wantArgs:    nil,
			wantOK:      true,
		},
		{
			name:        "cmd --verbose --chdir a (mixed)",
			words:       []string{"cmd", "--verbose", "--chdir", "a"},
			valueTaking: map[string]struct{}{"chdir": {}},
			wantFlags:   []string{"chdir", "verbose"},
			wantValues:  map[string][]string{"chdir": {"a"}},
			wantArgs:    nil,
			wantOK:      true,
		},
		{
			name:        "cmd -chdir (long, missing value at end)",
			words:       []string{"cmd", "-chdir"},
			valueTaking: map[string]struct{}{"chdir": {}},
			wantFlags:   []string{"chdir"},
			wantValues:  map[string][]string{},
			wantArgs:    nil,
			wantOK:      false,
		},
		{
			name:        "cmd -n (short, missing value at end)",
			words:       []string{"cmd", "-n"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n"},
			wantValues:  map[string][]string{},
			wantArgs:    nil,
			wantOK:      false,
		},
		{
			name:        "ls -la (unchanged when valueTaking empty)",
			words:       []string{"ls", "-la"},
			valueTaking: nil,
			wantFlags:   []string{"a", "l"},
			wantValues:  map[string][]string{},
			wantArgs:    nil,
			wantOK:      true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			flags, values, args, ok := flagnorm.Normalize(c.words, nil, c.valueTaking)
			if ok != c.wantOK {
				t.Errorf("ok: got %v, want %v", ok, c.wantOK)
			}
			if got := sortedKeys(flags); !reflect.DeepEqual(got, c.wantFlags) {
				t.Errorf("flags: got %v, want %v", got, c.wantFlags)
			}
			if !reflect.DeepEqual(normalizeMap(values), normalizeMap(c.wantValues)) {
				t.Errorf("values: got %v, want %v", values, c.wantValues)
			}
			if !reflect.DeepEqual(args, c.wantArgs) {
				t.Errorf("args: got %v, want %v", args, c.wantArgs)
			}
		})
	}
}

// normalizeMap returns nil for empty maps so [reflect.DeepEqual] works
// across (nil) vs (map[string][]string{}) constructed by Normalize.
func normalizeMap(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

func TestNormalize_AttachedAndBundledValueForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		words       []string
		valueTaking map[string]struct{}
		wantFlags   []string
		wantValues  map[string][]string
		wantArgs    []string
		wantOK      bool
	}{
		{
			name:        "cmd -nprod (1-char attached value)",
			words:       []string{"cmd", "-nprod"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n"},
			wantValues:  map[string][]string{"n": {"prod"}},
			wantArgs:    nil,
			wantOK:      true,
		},
		{
			name:        "cmd -vn prod (bundle hits n, consumes next)",
			words:       []string{"cmd", "-vn", "prod"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n", "v"},
			wantValues:  map[string][]string{"n": {"prod"}},
			wantArgs:    nil,
			wantOK:      true,
		},
		{
			name:        "cmd -vnprod (bundle then attached value)",
			words:       []string{"cmd", "-vnprod"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n", "v"},
			wantValues:  map[string][]string{"n": {"prod"}},
			wantArgs:    nil,
			wantOK:      true,
		},
		{
			name:        "cmd -nv prod (n value-taking, consumes attached v; prod stays positional)",
			words:       []string{"cmd", "-nv", "prod"},
			valueTaking: map[string]struct{}{"n": {}},
			wantFlags:   []string{"n"},
			wantValues:  map[string][]string{"n": {"v"}},
			wantArgs:    []string{"prod"},
			wantOK:      true,
		},
		{
			name:        "rm -rf /tmp/x (no value-taking, regression)",
			words:       []string{"rm", "-rf", "/tmp/x"},
			valueTaking: nil,
			wantFlags:   []string{"f", "r"},
			wantValues:  map[string][]string{},
			wantArgs:    []string{"/tmp/x"},
			wantOK:      true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			flags, values, args, ok := flagnorm.Normalize(c.words, nil, c.valueTaking)
			if ok != c.wantOK {
				t.Errorf("ok: got %v, want %v", ok, c.wantOK)
			}
			if got := sortedKeys(flags); !reflect.DeepEqual(got, c.wantFlags) {
				t.Errorf("flags: got %v, want %v", got, c.wantFlags)
			}
			if !reflect.DeepEqual(normalizeMap(values), normalizeMap(c.wantValues)) {
				t.Errorf("values: got %v, want %v", values, c.wantValues)
			}
			if !reflect.DeepEqual(args, c.wantArgs) {
				t.Errorf("args: got %v, want %v", args, c.wantArgs)
			}
		})
	}
}
