package clispec

// MergeCLISpecs returns a new map containing the union of base and
// override. Override values are merged additively into base entries:
//
//   - value_taking_flags: union of both lists (deduplicated)
//   - recurse.flags:      union of both lists (deduplicated)
//   - recurse.terminator: override OR base (boolean OR)
//   - recurse.subcommands: override entries take precedence,
//     base entries kept if not present in override
//
// Neither base nor override is mutated. CLI entries present only in
// one side are copied through.
func MergeCLISpecs(base, override map[string]*CLISpec) map[string]*CLISpec {
	out := make(map[string]*CLISpec, len(base)+len(override))
	for k, v := range base {
		out[k] = cloneSpec(v)
	}
	for k, ov := range override {
		if ov == nil {
			continue
		}
		if existing, ok := out[k]; ok {
			out[k] = mergeSpec(existing, ov)
		} else {
			out[k] = cloneSpec(ov)
		}
	}
	return out
}

func cloneSpec(s *CLISpec) *CLISpec {
	if s == nil {
		return nil
	}
	out := &CLISpec{}
	if len(s.ValueTakingFlags) > 0 {
		out.ValueTakingFlags = append([]string(nil), s.ValueTakingFlags...)
	}
	if s.Recurse != nil {
		out.Recurse = cloneRecurse(s.Recurse)
	}
	return out
}

func cloneRecurse(r *RecurseSpec) *RecurseSpec {
	if r == nil {
		return nil
	}
	out := &RecurseSpec{
		Terminator: r.Terminator,
	}
	if len(r.Flags) > 0 {
		out.Flags = append([]string(nil), r.Flags...)
	}
	if len(r.Subcommands) > 0 {
		out.Subcommands = make(map[string]*SubcommandRecurse, len(r.Subcommands))
		for k, v := range r.Subcommands {
			if v == nil {
				continue
			}
			out.Subcommands[k] = &SubcommandRecurse{Skip: v.Skip}
		}
	}
	return out
}

func mergeSpec(base, override *CLISpec) *CLISpec {
	out := cloneSpec(base)
	if out == nil {
		out = &CLISpec{}
	}
	out.ValueTakingFlags = dedupAppend(out.ValueTakingFlags, override.ValueTakingFlags)
	if override.Recurse == nil {
		return out
	}
	if out.Recurse == nil {
		out.Recurse = &RecurseSpec{}
	}
	mergeRecurse(out.Recurse, override.Recurse)
	return out
}

func mergeRecurse(dst, override *RecurseSpec) {
	dst.Flags = dedupAppend(dst.Flags, override.Flags)
	dst.Terminator = dst.Terminator || override.Terminator
	if len(override.Subcommands) == 0 {
		return
	}
	if dst.Subcommands == nil {
		dst.Subcommands = map[string]*SubcommandRecurse{}
	}
	for verb, sub := range override.Subcommands {
		if sub == nil {
			continue
		}
		dst.Subcommands[verb] = &SubcommandRecurse{Skip: sub.Skip}
	}
}

func dedupAppend(base, override []string) []string {
	seen := make(map[string]struct{}, len(base)+len(override))
	out := make([]string, 0, len(base)+len(override))
	for _, v := range base {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range override {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
